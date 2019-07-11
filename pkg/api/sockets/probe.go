package sockets

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/googollee/go-socket.io"
	"github.com/grafana/metrictank/stats"
	"github.com/hashicorp/go-version"
	"github.com/raintank/tsdb-gw/auth"
	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/util"
	schemaV0 "gopkg.in/raintank/schema.v0"
	"gopkg.in/raintank/schema.v1"
)

var (
	RefreshDuration = stats.NewMeter32("api.probes.refresh-duration", true)
	metricsRecvd    = stats.NewCounter32("api.probes.metrics-recv")
)

type ProbeSocket struct {
	sync.Mutex
	*auth.User
	Probe             *m.ProbeDTO
	Socket            socketio.Socket
	Session           *m.ProbeSession
	closed            bool
	done              chan struct{}
	lastRefresh       time.Time
	refreshing        bool
	heartbeatInterval time.Duration
}

func NewProbeSocket(user *auth.User, probe *m.ProbeDTO, so socketio.Socket, session *m.ProbeSession, heartbeatInterval time.Duration) *ProbeSocket {
	return &ProbeSocket{
		User:              user,
		Probe:             probe,
		Socket:            so,
		Session:           session,
		done:              make(chan struct{}),
		heartbeatInterval: heartbeatInterval,
	}
}

func (p *ProbeSocket) EmitReady() error {
	log.Info("sending ready event to probeId %d", p.Probe.Id)
	readyPayload := &m.ProbeReadyPayload{
		Collector:    p.Probe,
		MonitorTypes: m.MonitorTypes,
		SocketId:     p.Session.SocketId,
	}
	return p.emit("ready", readyPayload)
}

func (p *ProbeSocket) emit(message string, payload interface{}) error {
	p.Lock()
	err := p.Socket.Emit(message, payload)
	p.Unlock()
	return err
}

func (p *ProbeSocket) Remove() error {
	log.Info("removing socket with Id %s for probeId=%d", p.Session.SocketId, p.Probe.Id)
	err := sqlstore.DeleteProbeSession(p.Session)
	if err != nil {

	}
	log.Info("probe session deleted from db for probeId=%d", p.Probe.Id)
	return err
}

func (p *ProbeSocket) OnDisconnection() {
	p.Lock()
	defer p.Unlock()
	if !p.closed {
		p.closed = true
		close(p.done)
	}
	log.Info("%s disconnected", p.Probe.Name)
	Remove(p.Session.SocketId)
	if err := p.Remove(); err != nil {
		log.Error(3, "Failed to remove probeSession for probeId=%d, %s", p.Probe.Id, err)
	}
}

func (p *ProbeSocket) OnEvent(msg *schema.ProbeEvent) {
	log.Debug("received event from probeId%", p.Probe.Id)
	if !p.Probe.Public {
		msg.OrgId = int64(p.User.ID)
	}
	publisher.AddEvent(msg)
}

func (p *ProbeSocket) OnResults(results []*schemaV0.MetricData) {
	metricsRecvd.Add(len(results))
	metrics := make([]*schema.MetricData, len(results))
	for i, m := range results {
		metrics[i] = &schema.MetricData{
			Name:     strings.Replace(m.Name, "litmus.", "worldping.", 1),
			Metric:   strings.Replace(m.Metric, "litmus.", "worldping.", 1),
			Interval: m.Interval,
			OrgId:    m.OrgId,
			Value:    m.Value,
			Time:     m.Time,
			Unit:     m.Unit,
			Mtype:    m.TargetType,
			Tags:     m.Tags,
		}
		metrics[i].SetId()

		if !p.Probe.Public {
			metrics[i].OrgId = p.User.ID
		}
	}
	publisher.Add(metrics)
}

func (p *ProbeSocket) Refresh() {
	p.Lock()
	if p.closed {
		p.Unlock()
		log.Info("Refresh called on closed session.")
		return
	}
	pre := time.Now()
	refreshInProgress := p.refreshing
	if !refreshInProgress {
		p.refreshing = true
		p.lastRefresh = pre
	}
	p.Unlock()
	if refreshInProgress {
		log.Info("probeId=%d (%s) ignoring refresh request as one is already running", p.Probe.Id, p.Probe.Name)
		return
	}
	defer func() {
		p.Lock()
		p.refreshing = false
		p.Unlock()
	}()
	log.Info("probeId=%d (%s) refreshing", p.Probe.Id, p.Probe.Name)
	//step 1. get list of collectorSessions for this collector.
	sessions, err := sqlstore.GetProbeSessions(p.Probe.Id, "", time.Now().Add(-2*p.heartbeatInterval))
	if err != nil {
		log.Error(3, "failed to get list of probeSessions.", err)
		return
	}

	totalSessions := int64(len(sessions))
	log.Debug("probeId=%d has %d sessions", p.Probe.Id, totalSessions)
	//step 2. for each session
	for pos, sess := range sessions {
		//we only need to refresh the 1 socket.
		if sess.SocketId != p.Session.SocketId {
			continue
		}
		//step 3. get list of checks configured for this colletor.
		checks, err := sqlstore.GetProbeChecksWithEndpointSlug(p.Probe)
		if err != nil {
			log.Error(3, "failed to get checks for probeId=%d. %s", p.Probe.Id, err)
			break
		}

		v, _ := version.NewVersion(p.Session.Version)
		newVer, _ := version.NewVersion("0.9.1")
		activeChecks := make([]m.CheckWithSlug, 0)
		monitors := make([]m.MonitorDTO, 0)
		for _, check := range checks {
			if !check.Enabled {
				continue
			}
			if check.Check.Id%totalSessions == int64(pos) {
				if v.LessThan(newVer) {
					monitors = append(monitors, m.MonitorDTOFromCheck(check.Check, check.Slug))
				} else {
					activeChecks = append(activeChecks, check)
				}
			}

		}

		if v.LessThan(newVer) {
			log.Info("sending refresh to socket %s, probeId=%d,  %d checks", sess.SocketId, sess.ProbeId, len(monitors))
			p.emit("refresh", monitors)
		} else {
			log.Info("sending refresh to socket %s, probeId=%d,  %d checks", sess.SocketId, sess.ProbeId, len(activeChecks))
			p.emit("refresh", activeChecks)
		}
		break
	}
	RefreshDuration.Value(util.Since(pre))
}

func (p *ProbeSocket) Start() {
	Set(p.Socket.Id(), p)
	log.Info("added probeSocket to socketCache for probeId=%d socketId=%s", p.Probe.Id, p.Socket.Id())
	log.Info("connection for probeId=%d registered without error", p.Probe.Id)
	if err := p.EmitReady(); err != nil {
		log.Error(3, "failed to send ready event to probe. %s", err)
		return
	}
	log.Info("binding event handlers for probeId=%d", p.Probe.Id)
	p.Socket.On("event", p.OnEvent)
	p.Socket.On("results", p.OnResults)
	p.Socket.On("disconnection", p.OnDisconnection)

	log.Info("saving probe session to DB for probeId=%d", p.Probe.Id)
	// adding the probeSession will emit an event that will trigger
	// a refresh to be sent.
	err := sqlstore.AddProbeSession(p.Session)
	if err != nil {
		log.Error(3, "Failed to add probeSession to DB.", err)
		p.emit("error", fmt.Sprintf("internal server error. %s", err.Error()))
		return
	}
	log.Info("saved session to DB for probeId=%d", p.Probe.Id)

	// write heartbeats to the DB
	go func() {
		ticker := time.NewTicker(p.heartbeatInterval)
		for {
			select {
			case <-ticker.C:
				p.Lock()
				p.Session.Updated = time.Now()
				p.Unlock()
				err = sqlstore.UpdateProbeSession(p.Session)
				if err != nil {
					log.Error(3, "Failed to update probeSession in DB. probeId=%d", p.Probe.Id, err)
				}
			case <-p.done:
				// probe disconnected.
				ticker.Stop()
				return
			}
		}
	}()
}

func (p *ProbeSocket) LastRefresh() time.Time {
	p.Lock()
	l := p.lastRefresh
	p.Unlock()
	return l
}
