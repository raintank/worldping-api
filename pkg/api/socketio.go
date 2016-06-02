package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fiorix/freegeoip"
	"github.com/googollee/go-socket.io"
	"github.com/grafana/grafana/pkg/log"
	"github.com/raintank/met"
	"github.com/raintank/raintank-apps/pkg/auth"
	"github.com/raintank/raintank-metric/schema"
	"github.com/raintank/worldping-api/pkg/events"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/collectoreventpublisher"
	"github.com/raintank/worldping-api/pkg/services/metricpublisher"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
	"github.com/raintank/worldping-api/pkg/util"
)

var server *socketio.Server
var contextCache *ContextCache
var metricsRecvd met.Count
var geoipDB *freegeoip.DB

type ContextCache struct {
	sync.RWMutex
	Contexts map[string]*CollectorContext
}

func (s *ContextCache) Set(id string, context *CollectorContext) {
	s.Lock()
	s.Contexts[id] = context
	s.Unlock()
}

func (s *ContextCache) Remove(id string) {
	s.Lock()
	delete(s.Contexts, id)
	s.Unlock()
}

func (s *ContextCache) Emit(id string, event string, payload interface{}) {
	s.RLock()
	defer s.RUnlock()
	context, ok := s.Contexts[id]
	if !ok {
		log.Info("socket " + id + " is not local.")
		return
	}
	context.Socket.Emit(event, payload)
}

func (c *ContextCache) Refresh(collectorId int64) {
	c.RLock()
	defer c.RUnlock()

	for _, ctx := range c.Contexts {
		if ctx.Probe.Id == collectorId {
			ctx.Refresh()
		}
	}
}

func (c *ContextCache) RefreshLoop() {
	ticker := time.NewTicker(time.Minute * 5)
	for {
		select {
		case <-ticker.C:
			c.RLock()
			for _, ctx := range c.Contexts {
				ctx.Refresh()
			}
			c.RUnlock()
		}
	}
}

func NewContextCache() *ContextCache {
	cache := &ContextCache{
		Contexts: make(map[string]*CollectorContext),
	}

	go cache.RefreshLoop()
	return cache
}

type CollectorContext struct {
	*auth.SignedInUser
	Probe   *m.ProbeDTO
	Socket  socketio.Socket
	Session *m.ProbeSession
}

func register(so socketio.Socket) (*CollectorContext, error) {
	req := so.Request()
	req.ParseForm()
	keyString := req.Form.Get("apiKey")
	name := req.Form.Get("name")
	if name == "" {
		return nil, errors.New("probe name not provided.")
	}

	lastSocketId := req.Form.Get("lastSocketId")

	versionStr := req.Form.Get("version")
	if versionStr == "" {
		return nil, errors.New("version number not provided.")
	}
	versionParts := strings.SplitN(versionStr, ".", 2)
	if len(versionParts) != 2 {
		return nil, errors.New("could not parse version number")
	}
	versionMajor, err := strconv.ParseInt(versionParts[0], 10, 64)
	if err != nil {
		return nil, errors.New("could not parse version number")
	}
	versionMinor, err := strconv.ParseFloat(versionParts[1], 64)
	if err != nil {
		return nil, errors.New("could not parse version number.")
	}

	//--------- set required version of collector.------------//
	//
	if versionMajor < 0 || versionMinor < 1.3 {
		return nil, errors.New("invalid probe version. Please upgrade.")
	}
	//
	//--------- set required version of collector.------------//
	log.Info("probe %s with version %s connected", name, versionStr)
	if keyString != "" {
		user, err := auth.Auth(setting.AdminKey, keyString)
		if err != nil {
			return nil, err
		}

		// lookup collector
		probe, err := sqlstore.GetProbeByName(name, user.OrgId)
		if err == m.ErrProbeNotFound {
			//collector not found, so lets create a new one.
			probe = &m.ProbeDTO{
				OrgId:   user.OrgId,
				Name:    name,
				Enabled: true,
			}

			err = sqlstore.AddProbe(probe)
			if err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
		remoteIp := net.ParseIP(util.GetRemoteIp(so.Request()))
		if remoteIp == nil {
			log.Error(3, "Unable to lookup remote IP address of probe.")
			remoteIp = net.ParseIP("0.0.0.0")
		} else if probe.Latitude == 0 || probe.Longitude == 0 {
			var location freegeoip.DefaultQuery
			err := geoipDB.Lookup(remoteIp, &location)
			if err != nil {
				log.Error(3, "Unabled to get location from IP.", err)
				return nil, err
			}
			probe.Latitude = location.Location.Latitude
			probe.Longitude = location.Location.Longitude
			log.Debug("probe %s is located at lat:%f, long:%f", probe.Name, location.Location.Latitude, location.Location.Longitude)

			if err := sqlstore.UpdateProbe(probe); err != nil {
				log.Error(3, "could not save Probe location to DB.", err)
				return nil, err
			}
		}

		sess := &CollectorContext{
			SignedInUser: user,
			Probe:        probe,
			Socket:       so,
			Session: &m.ProbeSession{
				OrgId:      user.OrgId,
				ProbeId:    probe.Id,
				SocketId:   so.Id(),
				Version:    versionStr,
				InstanceId: setting.InstanceId,
				RemoteIp:   remoteIp.String(),
			},
		}
		err = sqlstore.AddProbeSession(sess.Session)
		if err != nil {
			return nil, err
		}
		log.Info("probe %s with id %d owned by %d authenticated successfully from %s.", name, probe.Id, user.OrgId, remoteIp.String())
		if lastSocketId != "" {
			log.Info("removing socket with Id %s", lastSocketId)
			if err := sqlstore.DeleteProbeSession(&m.ProbeSession{OrgId: sess.OrgId, SocketId: lastSocketId, ProbeId: sess.Probe.Id}); err != nil {
				log.Error(3, "failed to remove probes lastSocketId", err)
				return nil, err
			}
		}

		log.Info("saving session to contextCache")
		contextCache.Set(sess.Session.SocketId, sess)
		log.Info("session saved to contextCache")
		return sess, nil
	}
	return nil, auth.ErrInvalidApiKey
}

func InitCollectorController(metrics met.Backend) {
	if err := sqlstore.ClearProbeSessions(setting.InstanceId); err != nil {
		log.Fatal(4, "failed to clear collectorSessions", err)
	}

	metricsRecvd = metrics.NewCount("collector-ctrl.metrics-recv")

	channel := make(chan events.RawEvent, 100)
	events.Subscribe("Endpoint.created", channel)
	events.Subscribe("Endpoint.updated", channel)
	events.Subscribe("Endpoint.deleted", channel)
	events.Subscribe("ProbeSession.created", channel)
	events.Subscribe("ProbeSession.deleted", channel)
	events.Subscribe("Probe.updated", channel)
	go eventConsumer(channel)

	metricsRecvd = metrics.NewCount("collector-ctrl.metrics-recv")

	// init GEOIP DB.
	var err error
	geoipDB, err = freegeoip.OpenURL(freegeoip.MaxMindDB, time.Hour, time.Hour*6)
	if err != nil {
		log.Error(3, "failed to load GEOIP DB. ", err)
	}

}

func init() {
	contextCache = NewContextCache()
	var err error
	server, err = socketio.NewServer([]string{"websocket"})
	if err != nil {
		log.Fatal(4, "failed to initialize socketio.", err)
		return
	}
	server.On("connection", func(so socketio.Socket) {
		c, err := register(so)
		if err != nil {
			if err == auth.ErrInvalidApiKey {
				log.Info("probe failed to authenticate.")
			} else if err.Error() == "invalid probe version. Please upgrade." {
				log.Info("probe is wrong version")
			} else {
				log.Error(3, "Failed to initialize probe.", err)
			}
			so.Emit("error", err.Error())
			return
		}
		log.Info("connection registered without error")
		if err := c.EmitReady(); err != nil {
			return
		}
		log.Info("binding event handlers for probe %s owned by OrgId: %d", c.Probe.Name, c.OrgId)
		if c.Session.Version == "0.1.3" {
			c.Socket.On("event", c.OnEventOld)
		} else {
			c.Socket.On("event", c.OnEvent)
		}
		c.Socket.On("results", c.OnResults)
		c.Socket.On("disconnection", c.OnDisconnection)

		log.Info("calling refresh for probe %s owned by OrgId: %d", c.Probe.Name, c.OrgId)
		time.Sleep(time.Second)
		c.Refresh()
	})

	server.On("error", func(so socketio.Socket, err error) {
		log.Error(3, "socket emitted error", err)
	})

}

func (c *CollectorContext) EmitReady() error {

	log.Info("sending ready event to probe %s", c.Probe.Name)
	readyPayload := map[string]interface{}{
		"collector":     c.Probe,
		"monitor_types": m.MonitorTypes,
		"socket_id":     c.Session.SocketId,
	}
	c.Socket.Emit("ready", readyPayload)
	return nil
}

func (c *CollectorContext) Remove() error {
	log.Info(fmt.Sprintf("removing socket with Id %s", c.Session.SocketId))
	err := sqlstore.DeleteProbeSession(c.Session)
	return err
}

func (c *CollectorContext) OnDisconnection() {
	log.Info(fmt.Sprintf("%s disconnected", c.Probe.Name))
	if err := c.Remove(); err != nil {
		log.Error(3, fmt.Sprintf("Failed to remove probeSession. %s", c.Probe.Name), err)
	}
	contextCache.Remove(c.Session.SocketId)
}

func (c *CollectorContext) OnEvent(msg *schema.ProbeEvent) {
	log.Debug(fmt.Sprintf("received event from %s", c.Probe.Name))
	if !c.Probe.Public {
		msg.OrgId = c.OrgId
	}

	if err := collectoreventpublisher.Publish(msg); err != nil {
		log.Error(0, "failed to publish event.", err)
	}
}

/* handle old eventFormat.*/
type probeEventOld struct {
	Id        string   `json:"id"`
	EventType string   `json:"event_type"`
	OrgId     int64    `json:"org_id"`
	Severity  string   `json:"severity"`
	Source    string   `json:"source"`
	Timestamp int64    `json:"timestamp"`
	Message   string   `json:"message"`
	Tags      []string `json:"tags"`
}

func (c *CollectorContext) OnEventOld(msg *probeEventOld) {
	log.Debug("received event from %s", c.Probe.Name)
	if !c.Probe.Public {
		msg.OrgId = c.OrgId
	}
	//convert our []string of key:valy pairs to
	// map[string]string
	tags := make(map[string]string)
	for _, t := range msg.Tags {
		parts := strings.SplitN(t, ":", 2)
		tags[parts[0]] = parts[1]
	}
	e := &schema.ProbeEvent{
		Id:        msg.Id,
		EventType: msg.EventType,
		OrgId:     msg.OrgId,
		Severity:  msg.Severity,
		Source:    msg.Source,
		Timestamp: msg.Timestamp,
		Message:   msg.Message,
		Tags:      tags,
	}
	if err := collectoreventpublisher.Publish(e); err != nil {
		log.Error(3, "failed to publish event.", err)
	}
}

func (c *CollectorContext) OnResults(results []*schema.MetricData) {
	metricsRecvd.Inc(int64(len(results)))
	for _, r := range results {
		r.SetId()
		if !c.Probe.Public {
			r.OrgId = int(c.OrgId)
		}
	}
	metricpublisher.Publish(results)
}

func (c *CollectorContext) Refresh() {
	log.Info("Probe %d (%s) refreshing", c.Probe.Id, c.Probe.Name)
	//step 1. get list of collectorSessions for this collector.
	sessions, err := sqlstore.GetProbeSessions(c.Probe.Id, "")
	if err != nil {
		log.Error(3, "failed to get list of probeSessions.", err)
		return
	}

	totalSessions := int64(len(sessions))
	//step 2. for each session
	for pos, sess := range sessions {
		//we only need to refresh the 1 socket.
		if sess.SocketId != c.Session.SocketId {
			continue
		}
		//step 3. get list of monitors configured for this colletor.
		checks, err := sqlstore.GetProbeChecksWithEndpointSlug(c.Probe)
		if err != nil {
			log.Error(3, "failed to get checks for probe %s. %s", c.Probe.Id, err)
			break
		}

		//step 5. send to socket.
		monitors := make([]m.MonitorDTO, 0, len(checks))
		for _, check := range checks {
			if check.Check.Id%totalSessions == int64(pos) {
				monitors = append(monitors, m.MonitorDTOFromCheck(check.Check, check.Slug))
			}
		}
		log.Info("sending refresh to %s. %d checks", sess.SocketId, len(monitors))
		c.Socket.Emit("refresh", monitors)
		break
	}
}

func SocketIO(c *middleware.Context) {
	if server == nil {
		log.Fatal(4, "socket.io server not initialized.", nil)
	}

	server.ServeHTTP(c.Resp, c.Req.Request)
}

func HandleEndpointUpdated(event *events.EndpointUpdated) error {
	log.Debug("processing EndpointUpdated event. EndpointId: %d", event.Payload.Current.Id)
	seenChecks := make(map[int64]struct{})
	oldChecks := make(map[int64]m.Check)
	changedChecks := make([]m.Check, 0)
	newChecks := make([]m.Check, 0)

	for _, check := range event.Payload.Last.Checks {
		oldChecks[check.Id] = check
	}

	// find the checks that have changed.
	for _, check := range event.Payload.Current.Checks {
		seenChecks[check.Id] = struct{}{}
		if check.Updated.After(event.Payload.Current.Updated) {
			if _, ok := oldChecks[check.Id]; ok {
				changedChecks = append(changedChecks, check)
			} else {
				newChecks = append(newChecks, check)
			}
		}
	}

	for _, check := range changedChecks {
		probeIds, err := sqlstore.GetProbesForCheck(&check)
		if err != nil {
			return err
		}
		seenProbes := make(map[int64]struct{})
		for _, probe := range probeIds {
			seenProbes[probe] = struct{}{}
			log.Debug("notifying probe:%d about updated %s check for %s", probe, check.Type, event.Payload.Current.Slug)
			if err := EmitCheckEvent(probe, check.Id, "updated", m.MonitorDTOFromCheck(check, event.Payload.Current.Slug)); err != nil {
				return err
			}
		}
		oldCheck := oldChecks[check.Id]
		oldProbes, err := sqlstore.GetProbesForCheck(&oldCheck)
		if err != nil {
			return err
		}
		for _, probe := range oldProbes {
			if _, ok := seenProbes[probe]; !ok {
				log.Debug("%s check for %s should no longer be running on probe %s", check.Type, event.Payload.Current.Slug, probe)
				if err := EmitCheckEvent(probe, check.Id, "removed", m.MonitorDTOFromCheck(check, event.Payload.Last.Slug)); err != nil {
					return err
				}
			}
		}
	}

	for _, check := range newChecks {
		probeIds, err := sqlstore.GetProbesForCheck(&check)
		if err != nil {
			return err
		}
		for _, probe := range probeIds {
			log.Debug("notifying probe:%d about new %s check for %s", probe, check.Type, event.Payload.Current.Slug)
			if err := EmitCheckEvent(probe, check.Id, "created", m.MonitorDTOFromCheck(check, event.Payload.Current.Slug)); err != nil {
				return err
			}
		}
	}

	for _, check := range oldChecks {
		if _, ok := seenChecks[check.Id]; !ok {
			oldProbes, err := sqlstore.GetProbesForCheck(&check)
			if err != nil {
				return err
			}
			for _, probe := range oldProbes {
				log.Debug("%s check for %s should no longer be running on probe %s", check.Type, event.Payload.Current.Slug, probe)
				if err := EmitCheckEvent(probe, check.Id, "removed", m.MonitorDTOFromCheck(check, event.Payload.Last.Slug)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func HandleEndpointCreated(event *events.EndpointCreated) error {
	log.Debug("processing EndpointCreated event. EndpointId: %d", event.Payload.Id)
	for _, check := range event.Payload.Checks {
		probeIds, err := sqlstore.GetProbesForCheck(&check)
		if err != nil {
			return err
		}
		for _, probe := range probeIds {
			log.Debug("notifying probe:%d about new %s check for %s", probe, check.Type, event.Payload.Slug)
			if err := EmitCheckEvent(probe, check.Id, "created", m.MonitorDTOFromCheck(check, event.Payload.Slug)); err != nil {
				return err
			}
		}
	}
	return nil
}

func HandleEndpointDeleted(event *events.EndpointDeleted) error {
	log.Debug("processing EndpointDeleted event. EndpointId: %d", event.Payload.Id)

	for _, check := range event.Payload.Checks {
		probeIds, err := sqlstore.GetProbesForCheck(&check)
		if err != nil {
			return err
		}
		for _, probe := range probeIds {
			log.Debug("notifying probe:%d about deleted %s check for %s", probe, check.Type, event.Payload.Slug)
			if err := EmitCheckEvent(probe, check.Id, "removed", m.MonitorDTOFromCheck(check, event.Payload.Slug)); err != nil {
				return err
			}
		}
	}
	return nil
}

func EmitCheckEvent(probeId int64, checkId int64, eventName string, event interface{}) error {
	sessions, err := sqlstore.GetProbeSessions(probeId, "")
	if err != nil {
		log.Error(3, "failed to get list of probeSessions.", err)
		return err
	}
	totalSessions := int64(len(sessions))
	if totalSessions < 1 {
		return nil
	}

	log.Info(fmt.Sprintf("emitting %s event for CheckId %d totalSessions: %d", eventName, checkId, totalSessions))
	pos := checkId % totalSessions
	if sessions[pos].InstanceId == setting.InstanceId {
		socketId := sessions[pos].SocketId
		contextCache.Emit(socketId, eventName, event)
	}
	return nil
}

func HandleProbeSessionCreated(event *events.ProbeSessionCreated) error {
	contextCache.Refresh(event.Payload.ProbeId)
	return nil
}

func HandleProbeSessionDeleted(event *events.ProbeSessionDeleted) error {
	contextCache.Refresh(event.Payload.ProbeId)
	return nil
}

func HandleProbeUpdated(event *events.ProbeUpdated) error {
	contextCache.RLock()
	defer contextCache.RUnlock()
	// get list of local sockets for this collector.
	contexts := make([]*CollectorContext, 0)
	for _, ctx := range contextCache.Contexts {
		if ctx.Probe.Id == event.Payload.Current.Id {
			contexts = append(contexts, ctx)
		}
	}
	if len(contexts) > 0 {
		for _, ctx := range contexts {
			ctx.Probe = event.Payload.Current
			_ = ctx.EmitReady()
		}
	}

	return nil
}

func eventConsumer(channel chan events.RawEvent) {
	for msg := range channel {
		go func(e events.RawEvent) {
			log.Debug("handling event of type %s", e.Type)
			log.Debug("%s", e.Body)
			switch e.Type {
			case "Endpoint.updated":
				event := events.EndpointUpdated{}
				if err := json.Unmarshal(e.Body, &event.Payload); err != nil {
					log.Error(3, "unable to unmarshal payload into EndpointUpdated event.", err)
					break
				}
				if err := HandleEndpointUpdated(&event); err != nil {
					log.Error(3, "failed to emit EndpointUpdated event.", err)
					// this is bad, but as probes refresh every 5minutes, the changes will propagate.
				}
				break
			case "Endpoint.created":
				event := events.EndpointCreated{}
				if err := json.Unmarshal(e.Body, &event.Payload); err != nil {
					log.Error(3, "unable to unmarshal payload into EndpointUpdated event.", err)
					break
				}
				if err := HandleEndpointCreated(&event); err != nil {
					log.Error(3, "failed to emit EndpointCreated event.", err)
				}
				break
			case "Endpoint.deleted":
				event := events.EndpointDeleted{}
				if err := json.Unmarshal(e.Body, &event.Payload); err != nil {
					log.Error(3, "unable to unmarshal payload into EndpointDeleted event.", err)
					break
				}
				if err := HandleEndpointDeleted(&event); err != nil {
					log.Error(3, "failed to emit EndpointDeleted event.", err)
				}
				break
			case "ProbeSession.created":
				event := events.ProbeSessionCreated{}
				if err := json.Unmarshal(e.Body, &event.Payload); err != nil {
					log.Error(3, "unable to unmarshal payload into ProbeSessionCreated event.", err)
					break
				}
				if err := HandleProbeSessionCreated(&event); err != nil {
					log.Error(3, "failed to emit ProbeSessionCreated event.", err)
				}
				break
			case "ProbeSesssion.deleted":
				event := events.ProbeSessionDeleted{}
				if err := json.Unmarshal(e.Body, &event.Payload); err != nil {
					log.Error(3, "unable to unmarshal payload into ProbeSessionDeleted event.", err)
					break
				}
				if err := HandleProbeSessionDeleted(&event); err != nil {
					log.Error(3, "failed to emit ProbeSessionDeleted event.", err)
				}
				break
			case "Probe.updated":
				event := events.ProbeUpdated{}
				if err := json.Unmarshal(e.Body, &event.Payload); err != nil {
					log.Error(3, "unable to unmarshal payload into ProbeUpdated event.", err)
					break
				}
				if err := HandleProbeUpdated(&event); err != nil {
					log.Error(3, "failed to emit ProbeUpdated event.", err)
				}
				break
			}
		}(msg)
	}
}
