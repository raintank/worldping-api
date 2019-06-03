package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/fiorix/freegeoip"
	"github.com/googollee/go-socket.io"
	"github.com/grafana/metrictank/stats"
	"github.com/hashicorp/go-version"
	"github.com/raintank/tsdb-gw/auth"
	"github.com/raintank/worldping-api/pkg/api/sockets"
	"github.com/raintank/worldping-api/pkg/events"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
	"github.com/raintank/worldping-api/pkg/util"
)

var server *socketio.Server
var geoipDB *freegeoip.DB

var heartbeatInterval = time.Second * 30

var (
	UpdatesRecv = stats.NewCounter32("api.probes.updates-recv")
	CreatesRecv = stats.NewCounter32("api.probes.creates-recv")
	RemovesRecv = stats.NewCounter32("api.probes.removes-recv")

	ProbeSessionCreatedEventsSeen = stats.NewCounter32("api.probes.session-created-events")
	ProbeSessionDeletedEventsSeen = stats.NewCounter32("api.probes.session-deleted-events")
)

func authenticate(keyString string) (*auth.User, error) {
	if keyString != "" {
		return middleware.GetUser(setting.AdminKey, keyString)
	}
	return nil, auth.ErrInvalidCredentials
}

func register(so socketio.Socket) (*sockets.ProbeSocket, error) {
	req := so.Request()
	req.ParseForm()
	keyString := req.Form.Get("apiKey")

	user, err := authenticate(keyString)
	if err != nil {
		return nil, err
	}

	name := req.Form.Get("name")
	if name == "" {
		return nil, errors.New("probe name not provided")
	}

	lastSocketId := req.Form.Get("lastSocketId")

	versionStr := req.Form.Get("version")
	if versionStr == "" {
		return nil, errors.New("version number not provided")
	}
	v, err := version.NewVersion(versionStr)
	if err != nil {
		return nil, err
	}

	//--------- set required version of probe.------------//
	minVersion, _ := version.NewVersion("0.1.4")
	if v.LessThan(minVersion) {
		return nil, errors.New("invalid probe version. Please upgrade")
	}

	log.Info("probe %s with version %s connected", name, v.String())

	// lookup collector
	probe, err := sqlstore.GetProbeByName(name, int64(user.ID))
	if err == m.ErrProbeNotFound {
		//check quotas
		ctx := &middleware.Context{
			User: user,
		}
		reached, err := middleware.QuotaReached(ctx, "probe")
		if err != nil {
			return nil, err
		}
		if reached {
			return nil, errors.New("Probe cant be created due to quota restriction")
		}
		//collector not found, so lets create a new one.
		probe = &m.ProbeDTO{
			OrgId:        int64(user.ID),
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
		log.Error(3, "unable to lookup remote IP address of probeId=%d", probe.Id)
		remoteIp = net.ParseIP("0.0.0.0")
	} else if probe.Latitude == 0 || probe.Longitude == 0 {
		var location freegeoip.DefaultQuery
		err := geoipDB.Lookup(remoteIp, &location)
		if err != nil {
			log.Error(3, "unable to get location from IP.", err)
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
	sess := &m.ProbeSession{
		OrgId:      int64(user.ID),
		ProbeId:    probe.Id,
		SocketId:   so.Id(),
		Version:    versionStr,
		InstanceId: setting.InstanceId,
		RemoteIp:   remoteIp.String(),
	}
	sock := sockets.NewProbeSocket(user, probe, so, sess, heartbeatInterval)

	log.Info("probe %s with probeId=%d owned by %d authenticated successfully from %s.", name, probe.Id, user.ID, remoteIp.String())
	if lastSocketId != "" {
		if err := sqlstore.DeleteProbeSession(&m.ProbeSession{OrgId: sess.OrgId, SocketId: lastSocketId, ProbeId: probe.Id}); err != nil {
			log.Error(3, "failed to remove lastSocketId for probeId=%d", probe.Id, err)
			return nil, err
		}
		log.Info("removed previous socket with Id %s from probeId=%d", lastSocketId, probe.Id)
		//allow time for our change to propagate.
		time.Sleep(time.Second)
	}
	return sock, nil
}

func InitCollectorController(pub services.MetricsEventsPublisher) {
	if err := sqlstore.ClearProbeSessions(setting.InstanceId); err != nil {
		log.Fatal(4, "failed to clear collectorSessions", err)
	}

	sockets.InitCache(pub)

	channel := make(chan events.RawEvent, 100)
	events.Subscribe("Endpoint.created", channel)
	events.Subscribe("Endpoint.updated", channel)
	events.Subscribe("Endpoint.deleted", channel)
	events.Subscribe("ProbeSession.created", channel)
	events.Subscribe("ProbeSession.deleted", channel)
	events.Subscribe("Probe.updated", channel)
	go eventConsumer(channel)

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
		sock, err := register(so)
		if err != nil {
			if err == auth.ErrInvalidCredentials {
				log.Info("probe failed to authenticate.")
			} else if err.Error() == "invalid probe version. Please upgrade." {
				log.Info("probeId is wrong version")
			} else {
				log.Error(3, "Failed to initialize probe.", err)
			}
			so.Emit("error", err.Error())
			return
		}
		sock.Start()
	})

	server.On("error", func(so socketio.Socket, err error) {
		log.Error(3, "socket %s emitted error %s", so.Id(), err)
	})

}

func ShutdownController() {
	log.Info("shutting down collectorController")
	sockets.Shutdown()
	log.Info("collectorController shutdown.")
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
	sessions, err := sqlstore.GetProbeSessions(probeId, "", time.Now().Add(-2*heartbeatInterval))
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
				sockets.Emit(socketId, eventName, monitor)
				return nil
			}
		}
		socketId := sessions[pos].SocketId
		sockets.Emit(socketId, eventName, event)
	}
	return nil
}

func HandleProbeSessionCreated(event *events.ProbeSessionCreated) error {
	log.Info("ProbeSessionCreated on %s: ProbeId=%d", event.Payload.InstanceId, event.Payload.ProbeId)
	sockets.Refresh(event.Payload.ProbeId)
	return nil
}

func HandleProbeSessionDeleted(event *events.ProbeSessionDeleted) error {
	log.Info("ProbeSessionDeleted from %s: ProbeId=%d", event.Payload.InstanceId, event.Payload.ProbeId)
	sockets.Refresh(event.Payload.ProbeId)
	return nil
}

func HandleProbeUpdated(event *events.ProbeUpdated) error {
	sockets.UpdateProbe(event.Payload.Current)

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
				UpdatesRecv.Inc()
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
				CreatesRecv.Inc()
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
				RemovesRecv.Inc()
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
				ProbeSessionCreatedEventsSeen.Inc()
				if err := HandleProbeSessionCreated(&event); err != nil {
					log.Error(3, "failed to emit ProbeSessionCreated event.", err)
				}
				break
			case "ProbeSession.deleted":
				event := events.ProbeSessionDeleted{}
				if err := json.Unmarshal(e.Body, &event.Payload); err != nil {
					log.Error(3, "unable to unmarshal payload into ProbeSessionDeleted event.", err)
					break
				}
				ProbeSessionDeletedEventsSeen.Inc()

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
