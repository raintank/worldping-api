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
	"github.com/raintank/raintank-apps/pkg/auth"
	"github.com/raintank/worldping-api/pkg/events"
	m "github.com/raintank/worldping-api/pkg/models"
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

	Convey("When socket connected with invalid apiKey", t, func() {
		addr := fmt.Sprintf("ws://%s/socket.io/?EIO=3&transport=websocket&version=0.1.4&apiKey=badpass&name=test", serverAddr.Host)
		client, err := gosocketio.Dial(addr, transport.GetDefaultWebsocketTransport())
		So(err, ShouldBeNil)
		disconChan := make(chan struct{})
		errorChan := make(chan string)
		client.On("refresh", func(c *gosocketio.Channel, checks []m.MonitorDTO) {
			t.Fatal("received refresh event.")
		})
		client.On("ready", func(c *gosocketio.Channel, event ReadyPayload) {
			t.Fatal("received ready event.")
		})
		client.On("error", func(c *gosocketio.Channel, reason string) {
			t.Log("received error event.")
			errorChan <- reason
			client.Close()
		})
		client.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
			t.Log("client disconnected")
			disconChan <- struct{}{}
		})

		var disconnected, errorMsg bool

		timer := time.NewTimer(time.Second * 10)
	LOOP:
		for {
			select {
			case <-timer.C:
				t.Fatal("disconnect event didnt fire")
			case <-disconChan:
				disconnected = true
				if disconnected && errorMsg {
					timer.Stop()
					break LOOP
				}
			case reason := <-errorChan:
				errorMsg = true
				So(reason, ShouldEqual, auth.ErrInvalidApiKey.Error())
				if disconnected && errorMsg {
					timer.Stop()
					break LOOP
				}
			}
		}
	})

	Convey("When socket connected with invalid version", t, func() {
		addr := fmt.Sprintf("ws://%s/socket.io/?EIO=3&transport=websocket&version=0.1.2&apiKey=test&name=test", serverAddr.Host)
		client, err := gosocketio.Dial(addr, transport.GetDefaultWebsocketTransport())
		So(err, ShouldBeNil)
		disconChan := make(chan struct{})
		errorChan := make(chan string)
		client.On("refresh", func(c *gosocketio.Channel, checks []m.MonitorDTO) {
			t.Fatal("received refresh event.")
		})
		client.On("ready", func(c *gosocketio.Channel, event ReadyPayload) {
			t.Fatal("received ready event.")
		})
		client.On("error", func(c *gosocketio.Channel, reason string) {
			t.Log("received error event.")
			errorChan <- reason
			client.Close()
		})
		client.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
			t.Log("client disconnected")
			disconChan <- struct{}{}
		})

		var disconnected, errorMsg bool

		timer := time.NewTimer(time.Second * 10)
	LOOP:
		for {
			select {
			case <-timer.C:
				t.Fatal("disconnect event didnt fire")
			case <-disconChan:
				disconnected = true
				if disconnected && errorMsg {
					timer.Stop()
					break LOOP
				}
			case reason := <-errorChan:
				errorMsg = true
				So(reason, ShouldEqual, "invalid probe version. Please upgrade.")
				if disconnected && errorMsg {
					timer.Stop()
					break LOOP
				}
			}
		}
	})

	Convey("When socket connected with valid params", t, func() {
		addr := fmt.Sprintf("ws://%s/socket.io/?EIO=3&transport=websocket&version=0.1.4&apiKey=test&name=test", serverAddr.Host)
		client, err := gosocketio.Dial(addr, transport.GetDefaultWebsocketTransport())
		So(err, ShouldBeNil)
		refresh := make(chan []m.MonitorDTO)
		readyChan := make(chan ReadyPayload)
		createChan := make(chan m.MonitorDTO)
		updateChan := make(chan m.MonitorDTO)
		removeChan := make(chan m.MonitorDTO)
		client.On("refresh", func(c *gosocketio.Channel, checks []m.MonitorDTO) {
			t.Log("received refresh event.")
			refresh <- checks
		})
		client.On("ready", func(c *gosocketio.Channel, event ReadyPayload) {
			t.Log("received ready event.")
			readyChan <- event
		})
		client.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
			t.Log("client disconnected")
		})
		//error catching handler
		client.On(gosocketio.OnError, func(c *gosocketio.Channel) {
			t.Fatal("client error recieved.")
		})
		client.On("updated", func(c *gosocketio.Channel, check m.MonitorDTO) {
			updateChan <- check
		})
		client.On("created", func(c *gosocketio.Channel, check m.MonitorDTO) {
			createChan <- check
		})
		client.On("removed", func(c *gosocketio.Channel, check m.MonitorDTO) {
			removeChan <- check
		})

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
			readyEvent := <-readyChan
			So(len(readyEvent.MonitorTypes), ShouldEqual, 4)
			So(readyEvent.Collector.Id, ShouldEqual, 1)
			So(readyEvent.Collector.Name, ShouldEqual, "test")
			checkList := <-refresh
			So(len(checkList), ShouldEqual, 0)

			Convey("when new checks created", func() {
				endpoint := m.EndpointDTO{
					Name:  "www.google.com",
					OrgId: 1,
					Tags:  []string{"test", "foo"},
					Checks: []m.Check{
						{
							Route: &m.CheckRoute{
								Type: m.RouteByIds,
								Config: map[string]interface{}{
									"ids": []int64{1},
								},
							},
							Frequency: 60,
							Type:      m.HTTP_CHECK,
							Enabled:   true,
							Settings: map[string]interface{}{
								"host":    "www.google.com",
								"path":    "/",
								"port":    80,
								"method":  "GET",
								"timeout": 5,
							},
							HealthSettings: &m.CheckHealthSettings{
								NumProbes: 1,
								Steps:     3,
							},
						},
						{
							Route: &m.CheckRoute{
								Type: m.RouteByIds,
								Config: map[string]interface{}{
									"ids": []int64{1},
								},
							},
							Frequency: 60,
							Type:      m.PING_CHECK,
							Enabled:   true,
							Settings: map[string]interface{}{
								"hostname": "www.google.com",
								"timeout":  5,
							},
							HealthSettings: &m.CheckHealthSettings{
								NumProbes: 1,
								Steps:     3,
							},
						},
					},
				}
				err := sqlstore.AddEndpoint(&endpoint)
				So(err, ShouldBeNil)

				//should recieve two new checks
				newCheck := <-createChan
				So(newCheck.EndpointSlug, ShouldEqual, "www_google_com")
				newCheck = <-createChan
				So(newCheck.EndpointSlug, ShouldEqual, "www_google_com")

				Convey("when checks modified", func() {
					endpoint.Checks[0].Enabled = false
					endpoint.Checks[1].Settings["timeout"] = 10
					err := sqlstore.UpdateEndpoint(&endpoint)
					So(err, ShouldBeNil)
					updatedCheck := <-updateChan
					for _, setting := range updatedCheck.Settings {
						if setting.Variable == "timeout" {
							So(setting.Value, ShouldEqual, "10")
						}
					}
					removed := <-removeChan
					So(removed.Id, ShouldEqual, endpoint.Checks[0].Id)
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

		})
	})

	Convey("When socket connected with 0.9 version", t, func() {
		addr := fmt.Sprintf("ws://%s/socket.io/?EIO=3&transport=websocket&version=0.9.1&apiKey=test&name=test2", serverAddr.Host)
		client, err := gosocketio.Dial(addr, transport.GetDefaultWebsocketTransport())
		So(err, ShouldBeNil)
		refresh := make(chan []*m.CheckWithSlug)
		readyChan := make(chan ReadyPayload)
		createChan := make(chan m.CheckWithSlug)
		updateChan := make(chan m.CheckWithSlug)
		removeChan := make(chan m.CheckWithSlug)
		client.On("refresh", func(c *gosocketio.Channel, checks []*m.CheckWithSlug) {
			t.Log("received refresh event.")
			refresh <- checks
		})
		client.On("ready", func(c *gosocketio.Channel, event ReadyPayload) {
			t.Log("received ready event.")
			readyChan <- event
		})
		client.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
			t.Log("client disconnected")
		})
		//error catching handler
		client.On(gosocketio.OnError, func(c *gosocketio.Channel) {
			t.Fatal("client error recieved.")
		})
		client.On("updated", func(c *gosocketio.Channel, check m.CheckWithSlug) {
			updateChan <- check
		})
		client.On("created", func(c *gosocketio.Channel, check m.CheckWithSlug) {
			createChan <- check
		})
		client.On("removed", func(c *gosocketio.Channel, check m.CheckWithSlug) {
			removeChan <- check
		})

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
			probe, err := sqlstore.GetProbeById(2, 1)
			So(err, ShouldBeNil)
			So(probe, ShouldNotBeNil)
			So(probe.Name, ShouldEqual, "test2")
			So(probe.Online, ShouldEqual, true)
			readyEvent := <-readyChan
			So(len(readyEvent.MonitorTypes), ShouldEqual, 4)
			So(readyEvent.Collector.Id, ShouldEqual, 2)
			So(readyEvent.Collector.Name, ShouldEqual, "test2")
			checkList := <-refresh
			So(len(checkList), ShouldEqual, 0)

			Convey("when new checks created", func() {
				endpoint := m.EndpointDTO{
					Name:  "www2.google.com",
					OrgId: 1,
					Tags:  []string{"test", "foo"},
					Checks: []m.Check{
						{
							Route: &m.CheckRoute{
								Type: m.RouteByIds,
								Config: map[string]interface{}{
									"ids": []int64{2},
								},
							},
							Frequency: 60,
							Type:      m.HTTP_CHECK,
							Enabled:   true,
							Settings: map[string]interface{}{
								"host":    "www.google.com",
								"path":    "/",
								"port":    80,
								"method":  "GET",
								"timeout": 5,
							},
							HealthSettings: &m.CheckHealthSettings{
								NumProbes: 1,
								Steps:     3,
							},
						},
						{
							Route: &m.CheckRoute{
								Type: m.RouteByIds,
								Config: map[string]interface{}{
									"ids": []int64{2},
								},
							},
							Frequency: 60,
							Type:      m.PING_CHECK,
							Enabled:   true,
							Settings: map[string]interface{}{
								"hostname": "www.google.com",
								"timeout":  5,
							},
							HealthSettings: &m.CheckHealthSettings{
								NumProbes: 1,
								Steps:     3,
							},
						},
					},
				}
				err := sqlstore.AddEndpoint(&endpoint)
				So(err, ShouldBeNil)

				//should recieve two new checks
				newCheck := <-createChan
				So(newCheck.Slug, ShouldEqual, "www2_google_com")
				newCheck = <-createChan
				So(newCheck.Slug, ShouldEqual, "www2_google_com")

				Convey("when checks modified", func() {
					endpoint.Checks[0].Enabled = false
					endpoint.Checks[1].Settings["timeout"] = 10
					err := sqlstore.UpdateEndpoint(&endpoint)
					So(err, ShouldBeNil)
					updatedCheck := <-updateChan

					So(updatedCheck.Settings["timeout"], ShouldEqual, 10)

					removed := <-removeChan
					So(removed.Id, ShouldEqual, endpoint.Checks[0].Id)
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

		})
	})
}
