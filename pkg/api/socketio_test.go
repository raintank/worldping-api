package api

import (
	//"bytes"
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Unknwon/macaron"
	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
	"github.com/raintank/met/helper"
	"github.com/raintank/worldping-api/pkg/events"
	//m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
	. "github.com/smartystreets/goconvey/convey"
)

func TestProbeController(t *testing.T) {
	setting.AdminKey = "test"
	InitTestDB(t)

	backend, _ := helper.New(false, "", "standard", "", "")
	events.Init()
	InitCollectorController(backend)
	r := macaron.Classic()
	Register(r)
	srv := httptest.NewServer(r)
	serverAddr, _ := url.Parse(srv.URL)

	eventChan := make(chan events.RawEvent)
	events.Subscribe("ProbeSession.created", eventChan)
	events.Subscribe("ProbeSession.deleted", eventChan)

	Convey("When socket connected", t, func() {
		addr := fmt.Sprintf("ws://%s/socket.io/?EIO=3&transport=websocket&version=0.1.4&apiKey=test&name=test", serverAddr.Host)
		client, err := gosocketio.Dial(addr, transport.GetDefaultWebsocketTransport())
		So(err, ShouldBeNil)
		timer := time.NewTimer(time.Second)
	LOOP:
		for {
			select {
			case <-timer.C:
				t.Fatal("waited too long for ProbeSession.created event.")
				break LOOP
			case e := <-eventChan:
				So(e.Type, ShouldEqual, "ProbeSession.created")
				break LOOP
			}
		}
		Convey("Probe should be created and online", func() {
			probe, err := sqlstore.GetProbeById(1, 1)
			So(err, ShouldBeNil)
			So(probe, ShouldNotBeNil)
			So(probe.Name, ShouldEqual, "test")
			So(probe.Online, ShouldEqual, true)

			Convey("when socket closes", func() {
				client.Close()
				timer := time.NewTimer(5 * time.Second)
			LOOP:
				for {
					select {
					case <-timer.C:
						t.Fatal("waited too long for ProbeSession.deleted event.")
						break LOOP
					case e := <-eventChan:
						So(e.Type, ShouldEqual, "ProbeSession.deleted")
						break LOOP
					}
				}
				Convey("Probe should be created and offline", func() {
					probe, err := sqlstore.GetProbeById(1, 1)
					So(err, ShouldBeNil)
					So(probe, ShouldNotBeNil)
					So(probe.Name, ShouldEqual, "test")
					So(probe.Online, ShouldEqual, false)
				})
			})
		})
	})
}
