package events

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

type TestEvent struct {
	timestamp time.Time
	body      map[string]string
}

func (t *TestEvent) Type() string {
	return "test.event"
}

func (t *TestEvent) Body() ([]byte, error) {
	return json.Marshal(t.body)
}

func (t *TestEvent) Timestamp() time.Time {
	return t.timestamp
}

func TestEventPublish(t *testing.T) {
	pubChan = make(chan Message, 1)
	enabled = true
	hostname, _ := os.Hostname()

	Convey("When publishing event", t, func() {
		e := &TestEvent{
			timestamp: time.Unix(1231421123, 223),
			body:      map[string]string{"data": "test"},
		}

		err := Publish(e, 0)

		So(err, ShouldBeNil)

		// get the Message sent to the publishChanel
		// in normal operation this would be picked up by the
		// rabbitmq publisher.
		msg := <-pubChan

		So(msg, ShouldHaveSameTypeAs, Message{})
		So(msg.RoutingKey, ShouldEqual, e.Type())

		// make sure the message sent to the Publish channel is
		// correct.
		raw := RawEvent{}
		err = json.Unmarshal(msg.Payload, &raw)
		So(err, ShouldBeNil)
		So(raw.Type, ShouldEqual, e.Type())
		So(raw.Timestamp, ShouldEqual, e.Timestamp())
		So(raw.Attempts, ShouldEqual, 1)
		So(raw.Source, ShouldEqual, hostname)

		// make sure we can unmashal to the original event.
		resultE := &TestEvent{}
		err = json.Unmarshal(raw.Body, resultE)
		So(err, ShouldBeNil)
		So(*resultE, ShouldEqual, *e)
	})
}

func TestEventHandler(t *testing.T) {
	handlers = &Handlers{
		Listeners: make(map[string][]chan<- RawEvent),
	}
	Convey("When no handlers registered", t, func() {
		l := handlers.GetListeners("foo")
		So(l, ShouldHaveSameTypeAs, make([]chan<- RawEvent))
		So(len(l), ShouldEqual, 0)

		// add a handler for the "foo" event
		c := make(chan RawEvent, 1)
		handlers.Add("foo", c)

		Convey("When 1 handler registered", t, func() {
			l = handlers.GetListeners("foo")
			So(len(l), ShouldEqual, 1)

			Convey("When fetching non-existing handler key", t, func() {
				l = handlers.GetListeners("bar")
				So(len(l), ShouldEqual, 0)
			})
		})
	})
}

func TestEventSubcribe(t *testing.T) {
	Init("", "")
	hostname, _ := os.Hostname()

	// add a handler for the "foo" event
	c := make(chan RawEvent, 1)
	Subscribe("test.event", c)
	e := &TestEvent{
		timestamp: time.Unix(1231421123, 223),
		body:      map[string]string{"data": "test"},
	}

	Convey("when Publishing event", t, func() {
		err := Publish(e, 0)
		So(err, ShouldBeNil)
		Convey("after publishing Event", t, func() {
			timer := time.NewTimer(time.Second)
			var raw RawEvent
			for {
				select {
				case <-timer.C:
					panic("timed out waiting for event on channel")
				case raw <- c:
					timer.Stop()
					break
				}
			}
			So(raw.Type, ShouldEqual, e.Type())
			So(raw.Timestamp, ShouldEqual, e.Timestamp())
			So(raw.Source, ShouldEqual, hostname)

			Convey("When marshiling rawEvent to event", t, func() {
				evnt := TestEvent{}
				err := json.Unmarshal(raw.Body, &evnt)
				So(err, ShouldBeNil)
				So(evnt, ShouldEqual, *e)
			})
		})
	})
}
