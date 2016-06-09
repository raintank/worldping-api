package events

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
	. "github.com/smartystreets/goconvey/convey"
)

type TestEvent struct {
	Ts      time.Time
	Payload map[string]string
}

func (t *TestEvent) Type() string {
	return "test.event"
}

func (t *TestEvent) Body() ([]byte, error) {
	return json.Marshal(t.Payload)
}

func (t *TestEvent) Timestamp() time.Time {
	return t.Ts
}

func TestEventPublish(t *testing.T) {
	pubChan = make(chan Message, 1)
	hostname, _ := os.Hostname()
	handlers = &Handlers{
		Listeners: make(map[string][]chan<- RawEvent),
	}

	Convey("When publishing event", t, func() {
		e := &TestEvent{
			Ts:      time.Unix(1231421123, 0),
			Payload: map[string]string{"data": "test"},
		}

		err := Publish(e, 0)

		So(err, ShouldBeNil)

		// get the Message sent to the publishChanel
		// in normal operation this would be picked up by the
		// rabbitmq publisher.
		msg := <-pubChan

		So(msg, ShouldHaveSameTypeAs, Message{})
		So(msg.RoutingKey, ShouldEqual, e.Type())
		t.Log(string(msg.Payload))
		// make sure the message sent to the Publish channel is
		// correct.
		raw := RawEvent{}
		err = json.Unmarshal(msg.Payload, &raw)
		So(err, ShouldBeNil)
		So(raw.Type, ShouldEqual, e.Type())
		So(raw.Timestamp.Unix(), ShouldEqual, e.Timestamp().Unix())
		So(raw.Attempts, ShouldEqual, 1)
		So(raw.Source, ShouldEqual, hostname)

		// make sure we can unmashal to the original event.
		resultE := make(map[string]string)
		err = json.Unmarshal(raw.Body, &resultE)
		So(err, ShouldBeNil)
		So(resultE, ShouldResemble, e.Payload)
	})
}

func TestEventHandler(t *testing.T) {
	handlers = &Handlers{
		Listeners: make(map[string][]chan<- RawEvent),
	}
	Convey("When no handlers registered", t, func() {
		l := handlers.GetListeners("foo")
		So(l, ShouldHaveSameTypeAs, make([]chan<- RawEvent, 0))
		So(len(l), ShouldEqual, 0)

		// add a handler for the "foo" event
		c := make(chan RawEvent, 1)
		handlers.Add("foo", c)

		Convey("When 1 handler registered", func() {
			l = handlers.GetListeners("foo")
			So(len(l), ShouldEqual, 1)

			Convey("When fetching non-existing handler key", func() {
				l = handlers.GetListeners("bar")
				So(len(l), ShouldEqual, 0)
			})
		})
	})
}

func TestEventSubcribe(t *testing.T) {
	setting.Rabbitmq = setting.RabbitmqSettings{
		Enabled: false,
	}
	Init()
	hostname, _ := os.Hostname()

	// add a handler for the "foo" event
	c := make(chan RawEvent, 1)
	Subscribe("test.event", c)
	e := &TestEvent{
		Ts:      time.Unix(1231421123, 0),
		Payload: map[string]string{"data": "test"},
	}

	Convey("when Publishing event", t, func() {
		err := Publish(e, 0)
		So(err, ShouldBeNil)
		Convey("after publishing Event", func() {
			timer := time.NewTimer(time.Second)
			var raw RawEvent
		LOOP:
			for {
				select {
				case <-timer.C:
					panic("timed out waiting for event on channel")
				case raw = <-c:
					timer.Stop()
					break LOOP
				}
			}
			So(raw.Type, ShouldEqual, e.Type())
			So(raw.Timestamp.Unix(), ShouldEqual, e.Timestamp().Unix())
			So(raw.Source, ShouldEqual, hostname)

			Convey("When marshiling rawEvent to event", func() {
				data := make(map[string]string)
				err := json.Unmarshal(raw.Body, &data)
				So(err, ShouldBeNil)
				So(data, ShouldResemble, e.Payload)
			})
		})
	})
}

func TestEventTypes(t *testing.T) {
	Convey("When converting EndpointCreated event to RawEvent", t, func() {
		r, err := NewRawEventFromEvent(&EndpointCreated{
			Ts:      time.Now(),
			Payload: &m.EndpointDTO{Name: "test"},
		})
		So(err, ShouldBeNil)
		So(r, ShouldNotBeNil)
		So(r.Type, ShouldEqual, "Endpoint.created")
	})
	Convey("When converting EndpointUpdated event to RawEvent", t, func() {
		e := &EndpointUpdated{
			Ts: time.Now(),
		}
		e.Payload.Current = &m.EndpointDTO{Name: "test"}
		e.Payload.Last = &m.EndpointDTO{Name: "test2"}
		r, err := NewRawEventFromEvent(e)
		So(err, ShouldBeNil)
		So(r, ShouldNotBeNil)
		So(r.Type, ShouldEqual, "Endpoint.updated")
	})
	Convey("When converting EndpointDeleted event to RawEvent", t, func() {
		r, err := NewRawEventFromEvent(&EndpointDeleted{
			Ts:      time.Now(),
			Payload: &m.EndpointDTO{Name: "test"},
		})
		So(err, ShouldBeNil)
		So(r, ShouldNotBeNil)
		So(r.Type, ShouldEqual, "Endpoint.deleted")
	})

	Convey("When converting ProbeCreated event to RawEvent", t, func() {
		r, err := NewRawEventFromEvent(&ProbeCreated{
			Ts:      time.Now(),
			Payload: &m.ProbeDTO{Name: "test"},
		})
		So(err, ShouldBeNil)
		So(r, ShouldNotBeNil)
		So(r.Type, ShouldEqual, "Probe.created")
	})
	Convey("When converting ProbeUpdated event to RawEvent", t, func() {
		e := &ProbeUpdated{
			Ts: time.Now(),
		}
		e.Payload.Current = &m.ProbeDTO{Name: "test"}
		e.Payload.Last = &m.ProbeDTO{Name: "test2"}
		r, err := NewRawEventFromEvent(e)
		So(err, ShouldBeNil)
		So(r, ShouldNotBeNil)
		So(r.Type, ShouldEqual, "Probe.updated")
	})
	Convey("When converting ProbeDeleted event to RawEvent", t, func() {
		r, err := NewRawEventFromEvent(&ProbeDeleted{
			Ts:      time.Now(),
			Payload: &m.ProbeDTO{Name: "test"},
		})
		So(err, ShouldBeNil)
		So(r, ShouldNotBeNil)
		So(r.Type, ShouldEqual, "Probe.deleted")
	})
	Convey("When converting ProbeOnline event to RawEvent", t, func() {
		r, err := NewRawEventFromEvent(&ProbeOnline{
			Ts:      time.Now(),
			Payload: &m.ProbeDTO{Name: "test"},
		})
		So(err, ShouldBeNil)
		So(r, ShouldNotBeNil)
		So(r.Type, ShouldEqual, "Probe.online")
	})
	Convey("When converting ProbeOffline event to RawEvent", t, func() {
		r, err := NewRawEventFromEvent(&ProbeOffline{
			Ts:      time.Now(),
			Payload: &m.ProbeDTO{Name: "test"},
		})
		So(err, ShouldBeNil)
		So(r, ShouldNotBeNil)
		So(r.Type, ShouldEqual, "Probe.offline")
	})

	Convey("When converting ProbeSessionCreated event to RawEvent", t, func() {
		r, err := NewRawEventFromEvent(&ProbeSessionCreated{
			Ts:      time.Now(),
			Payload: &m.ProbeSession{SocketId: "test"},
		})
		So(err, ShouldBeNil)
		So(r, ShouldNotBeNil)
		So(r.Type, ShouldEqual, "ProbeSession.created")
	})

	Convey("When converting ProbeSessionDeleted event to RawEvent", t, func() {
		r, err := NewRawEventFromEvent(&ProbeSessionDeleted{
			Ts:      time.Now(),
			Payload: &m.ProbeSession{SocketId: "test"},
		})
		So(err, ShouldBeNil)
		So(r, ShouldNotBeNil)
		So(r.Type, ShouldEqual, "ProbeSession.deleted")
	})
}
