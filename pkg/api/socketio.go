package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/fiorix/freegeoip"
	"github.com/googollee/go-socket.io"
	"github.com/hashicorp/go-version"
	"github.com/raintank/met"
	"github.com/raintank/raintank-apps/pkg/auth"
	"github.com/raintank/tsdb-gw/event_publish"
	"github.com/raintank/tsdb-gw/metric_publish"
	"github.com/raintank/worldping-api/pkg/events"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
	"github.com/raintank/worldping-api/pkg/util"
	schemaV0 "gopkg.in/raintank/schema.v0"
	"gopkg.in/raintank/schema.v1"
)

var server *socketio.Server
var contextCache *ContextCache
var geoipDB *freegeoip.DB

var (
	metricsRecvd                  met.Count
	ProbesConnected               met.Gauge
	ProbeSessionCreatedEventsSeen met.Count
	ProbeSessionDeletedEventsSeen met.Count
	UpdatesSent                   met.Count
	CreatesSent                   met.Count
	RemovesSent                   met.Count
	UpdatesRecv                   met.Count
	CreatesRecv                   met.Count
	RemovesRecv                   met.Count
	RefreshDuration               met.Timer
)

type ContextCache struct {
	sync.RWMutex
	Contexts    map[string]*CollectorContext
	shutdown    chan struct{}
	refreshChan chan int64
}

func (s *ContextCache) Set(id string, context *CollectorContext) {
	s.Lock()
	s.Contexts[id] = context
	s.Unlock()
	ProbesConnected.Inc(1)
}

func (s *ContextCache) Remove(id string) {
	s.Lock()
	delete(s.Contexts, id)
	s.Unlock()
	ProbesConnected.Inc(-1)
}

func (c *ContextCache) Shutdown() {
	c.shutdown <- struct{}{}
	sessList := make([]*CollectorContext, 0)
	c.Lock()
	for _, ctx := range c.Contexts {
		sessList = append(sessList, ctx)
	}
	c.Contexts = make(map[string]*CollectorContext)
	c.Unlock()
	for _, ctx := range sessList {
		ctx.Remove()
	}
	return
}

func (s *ContextCache) Emit(id string, event string, payload interface{}) {
	s.RLock()
	context, ok := s.Contexts[id]
	if !ok {
		log.Info("socket " + id + " is not local.")
		s.RUnlock()
		return
	}
	s.RUnlock()
	context.Socket.Emit(event, payload)
	switch event {
	case "updated":
		UpdatesSent.Inc(1)
	case "created":
		CreatesSent.Inc(1)
	case "removed":
		RemovesSent.Inc(1)
	}
}

func (c *ContextCache) Refresh(collectorId int64) {
	c.refreshChan <- collectorId
}

func (c *ContextCache) refresh(collectorId int64) {
	sessList := make([]*CollectorContext, 0)
	c.RLock()
	for _, ctx := range c.Contexts {
		if ctx.Probe.Id == collectorId {
			sessList = append(sessList, ctx)
		}
	}
	c.RUnlock()
	for _, ctx := range sessList {
		ctx.Refresh()
	}

}

func (c *ContextCache) RefreshQueue() {
	ticker := time.NewTicker(time.Second * 2)
	buffer := make([]int64, 0)
	for {
		select {
		case <-c.shutdown:
			log.Info("RefreshQueue terminating due to shutdown signal.")
			ticker.Stop()
			return
		case <-ticker.C:
			if len(buffer) == 0 {
				break
			}
			log.Debug("processing %d queued probe refreshes.", len(buffer))
			collectorIds := make(map[int64]struct{})
			for _, id := range buffer {
				collectorIds[id] = struct{}{}
			}
			log.Debug("%d refreshes are for %d probes", len(buffer), len(collectorIds))
			for id := range collectorIds {
				c.refresh(id)
			}
			buffer = buffer[:0]
		case id := <-c.refreshChan:
			log.Debug("adding refresh of %d to buffer", id)
			buffer = append(buffer, id)
		}
	}
}

func (c *ContextCache) RefreshLoop() {
	ticker := time.NewTicker(time.Minute * 5)
	for {
		select {
		case <-c.shutdown:
			log.Info("RefreshLoop terminating due to shutdown signal.")
			ticker.Stop()
			return
		case <-ticker.C:
			sessList := make([]*CollectorContext, 0)
			c.RLock()
			for _, ctx := range c.Contexts {
				if time.Since(ctx.LastRefresh) >= time.Minute*5 {
					sessList = append(sessList, ctx)
				}
			}
			c.RUnlock()
			for _, ctx := range sessList {
				ctx.Refresh()
			}

		}
	}
}

func NewContextCache() *ContextCache {
	cache := &ContextCache{
		Contexts:    make(map[string]*CollectorContext),
		shutdown:    make(chan struct{}),
		refreshChan: make(chan int64, 100),
	}

	go cache.RefreshLoop()
	go cache.RefreshQueue()
	return cache
}

type CollectorContext struct {
	*auth.SignedInUser
	Probe       *m.ProbeDTO
	Socket      socketio.Socket
	Session     *m.ProbeSession
	closed      bool
	LastRefresh time.Time
}

func authenticate(keyString string) (*auth.SignedInUser, error) {
	if keyString != "" {
		return auth.Auth(setting.AdminKey, keyString)
	}
	return nil, auth.ErrInvalidApiKey
}

func register(so socketio.Socket) (*CollectorContext, error) {
	req := so.Request()
	req.ParseForm()
	keyString := req.Form.Get("apiKey")

	user, err := authenticate(keyString)
	if err != nil {
		return nil, err
	}

	name := req.Form.Get("name")
	if name == "" {
		return nil, errors.New("probe name not provided.")
	}

	lastSocketId := req.Form.Get("lastSocketId")

	versionStr := req.Form.Get("version")
	if versionStr == "" {
		return nil, errors.New("version number not provided.")
	}
	v, err := version.NewVersion(versionStr)
	if err != nil {
		return nil, err
	}

	//--------- set required version of probe.------------//
	minVersion, _ := version.NewVersion("0.1.4")
	if v.LessThan(minVersion) {
		return nil, errors.New("invalid probe version. Please upgrade.")
	}

	log.Info("probe %s with version %s connected", name, v.String())

	// lookup collector
	probe, err := sqlstore.GetProbeByName(name, user.OrgId)
	if err == m.ErrProbeNotFound {
		//check quotas
		ctx := &middleware.Context{
			SignedInUser: user,
		}
		reached, err := middleware.QuotaReached(ctx, "probe")
		if err != nil {
			return nil, err
		}
		if reached {
			return nil, errors.New("Probe cant be created due to quota restriction.")
		}
		//collector not found, so lets create a new one.
		probe = &m.ProbeDTO{
			OrgId:        user.OrgId,
			Name:         name,
			Enabled:      true,
			Online:       true,
			OnlineChange: time.Now(),
		}

		err = sqlstore.AddProbe(probe)
		if err != nil {
			return nil, err
		}
		log.Info("created new entry in DB for probe %s, probeId=%d", probe.Name, probe.Id)
	} else if err != nil {
		return nil, err
	}
	remoteIp := net.ParseIP(util.GetRemoteIp(so.Request()))
	if remoteIp == nil {
		log.Error(3, "Unable to lookup remote IP address of probeId=%d", probe.Id)
		remoteIp = net.ParseIP("0.0.0.0")
	} else if probe.Latitude == 0 || probe.Longitude == 0 {
		var location freegeoip.DefaultQuery
		err := geoipDB.Lookup(remoteIp, &location)
		if err != nil {
			log.Error(3, "Unabled to get location from IP.", err)
		} else {
			probe.Latitude = location.Location.Latitude
			probe.Longitude = location.Location.Longitude
			log.Debug("probe %s is located at lat:%f, long:%f", probe.Name, location.Location.Latitude, location.Location.Longitude)
			log.Info("updating location data for probeId=%d,  lat:%f, long:%f", probe.Id, location.Location.Latitude, location.Location.Longitude)
			if err := sqlstore.UpdateProbe(probe); err != nil {
				log.Error(3, "could not save Probe location to DB.", err)
				return nil, err
			}
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

	log.Info("probe %s with probeId=%d owned by %d authenticated successfully from %s.", name, probe.Id, user.OrgId, remoteIp.String())
	if lastSocketId != "" {
		if err := sqlstore.DeleteProbeSession(&m.ProbeSession{OrgId: sess.OrgId, SocketId: lastSocketId, ProbeId: sess.Probe.Id}); err != nil {
			log.Error(3, "failed to remove lastSocketId for probeId=%d", probe.Id, err)
			return nil, err
		}
		log.Info("removed previous socket with Id %s from probeId=%d", lastSocketId, sess.Probe.Id)
		//allow time for our change to propagate.
		time.Sleep(time.Second)
	}

	contextCache.Set(sess.Session.SocketId, sess)
	log.Info("saved session to contextCache for probeId=%d", probe.Id)
	return sess, nil
}

func InitCollectorController(metrics met.Backend) {
	if err := sqlstore.ClearProbeSessions(setting.InstanceId); err != nil {
		log.Fatal(4, "failed to clear collectorSessions", err)
	}

	contextCache = NewContextCache()

	channel := make(chan events.RawEvent, 100)
	events.Subscribe("Endpoint.created", channel)
	events.Subscribe("Endpoint.updated", channel)
	events.Subscribe("Endpoint.deleted", channel)
	events.Subscribe("ProbeSession.created", channel)
	events.Subscribe("ProbeSession.deleted", channel)
	events.Subscribe("Probe.updated", channel)
	go eventConsumer(channel)

	metricsRecvd = metrics.NewCount("collector-ctrl.metrics-recv")
	ProbesConnected = metrics.NewGauge("collector-ctrl.probes-connected", 0)

	UpdatesSent = metrics.NewCount("collector-ctrl.updates-sent")
	CreatesSent = metrics.NewCount("collector-ctrl.creates-sent")
	RemovesSent = metrics.NewCount("collector-ctrl.removes-sent")

	UpdatesRecv = metrics.NewCount("collector-ctrl.updates-recv")
	CreatesRecv = metrics.NewCount("collector-ctrl.creates-recv")
	RemovesRecv = metrics.NewCount("collector-ctrl.removes-recv")

	RefreshDuration = metrics.NewTimer("collector-ctrl.refresh-duration", 0)

	ProbeSessionCreatedEventsSeen = metrics.NewCount("collector-ctrl.probe-session-created-events")
	ProbeSessionDeletedEventsSeen = metrics.NewCount("collector-ctrl.probe-session-deleted-events")

	// init GEOIP DB.
	var err error
	geoipDB, err = freegeoip.OpenURL(freegeoip.MaxMindDB, time.Hour, time.Hour*6)
	if err != nil {
		log.Error(3, "failed to load GEOIP DB. ", err)
	}

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
				log.Info("probeIdis wrong version")
			} else {
				log.Error(3, "Failed to initialize probe.", err)
			}
			so.Emit("error", err.Error())
			return
		}
		log.Info("connection for probeId=%d registered without error", c.Probe.Id)
		if err := c.EmitReady(); err != nil {
			return
		}
		log.Info("binding event handlers for probeId=%d", c.Probe.Id)
		c.Socket.On("event", c.OnEvent)
		c.Socket.On("results", c.OnResults)
		c.Socket.On("disconnection", c.OnDisconnection)

		log.Info("saving probe session to DB for probeId=%d", c.Probe.Id)
		err = sqlstore.AddProbeSession(c.Session)
		if err != nil {
			log.Error(3, "Failed to add probeSession to DB.", err)
			so.Emit("error", fmt.Sprintf("internal server error. %s", err.Error()))
			return
		}
		log.Info("saved session to DB for probeId=%d", c.Probe.Id)
		// adding the probeSession will emit an event that will trigger
		// a refresh to be sent.
	})

	server.On("error", func(so socketio.Socket, err error) {
		log.Error(3, "socket emitted error", err)
	})

}

func ShutdownController() {
	log.Info("shutting down collectorController")
	contextCache.Shutdown()
	log.Info("collectorController shutdown.")
}

func (c *CollectorContext) EmitReady() error {
	log.Info("sending ready event to probeId %d", c.Probe.Id)
	readyPayload := &m.ProbeReadyPayload{
		Collector:    c.Probe,
		MonitorTypes: m.MonitorTypes,
		SocketId:     c.Session.SocketId,
	}
	c.Socket.Emit("ready", readyPayload)
	return nil
}

func (c *CollectorContext) Remove() error {
	log.Info("removing socket with Id %s for probeId=%d", c.Session.SocketId, c.Probe.Id)
	err := sqlstore.DeleteProbeSession(c.Session)
	log.Info("probe session deleted from db for probeId=%d", c.Probe.Id)
	return err
}

func (c *CollectorContext) OnDisconnection() {
	c.closed = true
	log.Info("%s disconnected", c.Probe.Name)
	contextCache.Remove(c.Session.SocketId)
	if err := c.Remove(); err != nil {
		log.Error(3, "Failed to remove probeSession for probeId=%d, %s", c.Probe.Id, err)
	}
}

func (c *CollectorContext) OnEvent(msg *schema.ProbeEvent) {
	log.Debug("received event from probeId%", c.Probe.Id)
	if !c.Probe.Public {
		msg.OrgId = c.OrgId
	}
	err := event_publish.Publish(msg)
	if err != nil {
		log.Error(3, "failed to publush Event. %s", err)
		return
	}
}

func (c *CollectorContext) OnResults(results []*schemaV0.MetricData) {
	metricsRecvd.Inc(int64(len(results)))
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

		if !c.Probe.Public {
			metrics[i].OrgId = int(c.OrgId)
		}
	}
	metric_publish.Publish(metrics)
}

func (c *CollectorContext) Refresh() {
	if c.closed {
		log.Info("Refresh called on closed session.")
		return
	}
	pre := time.Now()
	c.LastRefresh = pre
	log.Info("probeId=%d (%s) refreshing", c.Probe.Id, c.Probe.Name)
	//step 1. get list of collectorSessions for this collector.
	sessions, err := sqlstore.GetProbeSessions(c.Probe.Id, "")
	if err != nil {
		log.Error(3, "failed to get list of probeSessions.", err)
		return
	}

	totalSessions := int64(len(sessions))
	log.Debug("probeId=%d has %d sessions", c.Probe.Id, totalSessions)
	//step 2. for each session
	for pos, sess := range sessions {
		//we only need to refresh the 1 socket.
		if sess.SocketId != c.Session.SocketId {
			continue
		}
		//step 3. get list of checks configured for this colletor.
		checks, err := sqlstore.GetProbeChecksWithEndpointSlug(c.Probe)
		if err != nil {
			log.Error(3, "failed to get checks for probeId=%d. %s", c.Probe.Id, err)
			break
		}

		v, _ := version.NewVersion(c.Session.Version)
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
			c.Socket.Emit("refresh", monitors)
		} else {
			log.Info("sending refresh to socket %s, probeId=%d,  %d checks", sess.SocketId, sess.ProbeId, len(activeChecks))
			c.Socket.Emit("refresh", activeChecks)
		}
		break
	}
	RefreshDuration.Value(time.Since(pre))
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
		if check.Enabled {
			oldChecks[check.Id] = check
		}
	}

	// find the checks that have changed.
	for _, check := range event.Payload.Current.Checks {
		if check.Enabled {
			seenChecks[check.Id] = struct{}{}
			if !check.Updated.Before(event.Payload.Current.Updated) {
				if _, ok := oldChecks[check.Id]; ok {
					changedChecks = append(changedChecks, check)
				} else {
					newChecks = append(newChecks, check)
				}
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
			log.Debug("notifying probeId=%d about updated %s check for %s", probe, check.Type, event.Payload.Current.Slug)
			if err := EmitCheckEvent(probe, check.Id, "updated", m.CheckWithSlug{Check: check, Slug: event.Payload.Current.Slug}); err != nil {
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
				log.Debug("%s check for %s should no longer be running on probeId=%d", check.Type, event.Payload.Current.Slug, probe)
				if err := EmitCheckEvent(probe, check.Id, "removed", m.CheckWithSlug{Check: check, Slug: event.Payload.Last.Slug}); err != nil {
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
			log.Debug("notifying probeId=%d about new %s check for %s", probe, check.Type, event.Payload.Current.Slug)
			if err := EmitCheckEvent(probe, check.Id, "created", m.CheckWithSlug{Check: check, Slug: event.Payload.Current.Slug}); err != nil {
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
				log.Debug("%s check for %s should no longer be running on probeId=%d", check.Type, event.Payload.Current.Slug, probe)
				if err := EmitCheckEvent(probe, check.Id, "removed", m.CheckWithSlug{Check: check, Slug: event.Payload.Last.Slug}); err != nil {
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
		if !check.Enabled {
			continue
		}
		probeIds, err := sqlstore.GetProbesForCheck(&check)
		if err != nil {
			return err
		}
		for _, probe := range probeIds {
			log.Debug("notifying probeId=%d about new %s check for %s", probe, check.Type, event.Payload.Slug)
			if err := EmitCheckEvent(probe, check.Id, "created", m.CheckWithSlug{Check: check, Slug: event.Payload.Slug}); err != nil {
				return err
			}
		}
	}
	return nil
}

func HandleEndpointDeleted(event *events.EndpointDeleted) error {
	log.Debug("processing EndpointDeleted event. EndpointId: %d", event.Payload.Id)

	for _, check := range event.Payload.Checks {
		if !check.Enabled {
			continue
		}
		probeIds, err := sqlstore.GetProbesForCheck(&check)
		if err != nil {
			return err
		}
		for _, probe := range probeIds {
			log.Debug("notifying probeId=%d about deleted %s check for %s", probe, check.Type, event.Payload.Slug)
			if err := EmitCheckEvent(probe, check.Id, "removed", m.CheckWithSlug{Check: check, Slug: event.Payload.Slug}); err != nil {
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

	log.Info(fmt.Sprintf("emitting %s event for CheckId %d to probeId:%d totalSessions: %d", eventName, checkId, probeId, totalSessions))
	pos := checkId % totalSessions
	if sessions[pos].InstanceId == setting.InstanceId {
		v, _ := version.NewVersion(sessions[pos].Version)
		newVer, _ := version.NewVersion("0.9.1")
		if v.LessThan(newVer) {
			if check, ok := event.(m.CheckWithSlug); ok {
				monitor := m.MonitorDTOFromCheckWithSlug(check)
				socketId := sessions[pos].SocketId
				contextCache.Emit(socketId, eventName, monitor)
				return nil
			}
		}
		socketId := sessions[pos].SocketId
		contextCache.Emit(socketId, eventName, event)
	}
	return nil
}

func HandleProbeSessionCreated(event *events.ProbeSessionCreated) error {
	log.Info("ProbeSessionCreated on %s: ProbeId=%d", event.Payload.InstanceId, event.Payload.ProbeId)
	contextCache.Refresh(event.Payload.ProbeId)
	return nil
}

func HandleProbeSessionDeleted(event *events.ProbeSessionDeleted) error {
	log.Info("ProbeSessionDeleted from %s: ProbeId=%d", event.Payload.InstanceId, event.Payload.ProbeId)
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
				UpdatesRecv.Inc(1)
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
				CreatesRecv.Inc(1)
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
				RemovesRecv.Inc(1)
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
				ProbeSessionCreatedEventsSeen.Inc(1)
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
				ProbeSessionDeletedEventsSeen.Inc(1)

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
