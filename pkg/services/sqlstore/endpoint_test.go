package sqlstore

import (
	"fmt"
	"testing"
	"time"

	m "github.com/raintank/worldping-api/pkg/models"
	. "github.com/smartystreets/goconvey/convey"
)

func populateProbes(t *testing.T) {
	for _, i := range []int{1, 2, 3} {
		err := AddProbe(&m.ProbeDTO{
			Name:      fmt.Sprintf("test%d", i),
			OrgId:     1,
			Tags:      []string{"test", fmt.Sprintf("dev%d", i%2)},
			Public:    false,
			Latitude:  1.0,
			Longitude: 1.0,
			Online:    false,
			Enabled:   true,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, i := range []int{1, 2} {
		pubProbe := &m.ProbeDTO{
			Name:      fmt.Sprintf("public%d", i),
			OrgId:     2,
			Tags:      []string{"public"},
			Public:    true,
			Latitude:  1.0,
			Longitude: 1.0,
			Online:    false,
			Enabled:   true,
		}
		err := AddProbe(pubProbe)
		if err != nil {
			t.Fatal(err)
		}

		// add tags for orgid 1 for pub probes owned by orgId2
		pubProbe.OrgId = 1
		pubProbe.Tags = []string{"pTest", "test", "foo"}
		err = UpdateProbe(pubProbe)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestEndpoints(t *testing.T) {
	InitTestDB(t)
	populateProbes(t)
	endpointCount := 0
	Convey("When adding endpoint", t, func() {
		pre := time.Now()
		e := &m.EndpointDTO{
			Name:  fmt.Sprintf("www%d.google.com", endpointCount),
			OrgId: 1,
			Tags:  []string{"test", "dev"},
			Checks: []m.Check{
				{
					Route: &m.CheckRoute{
						Type: m.RouteByTags,
						Config: map[string]interface{}{
							"tags": []string{"test"},
						},
					},
					Frequency: 60,
					Type:      m.HTTP_CHECK,
					Enabled:   true,
					Settings: map[string]interface{}{
						"host":    fmt.Sprintf("www%d.google.com", endpointCount),
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
							"ids": []int64{1, 2, 4},
						},
					},
					Frequency: 60,
					Type:      m.PING_CHECK,
					Enabled:   true,
					Settings: map[string]interface{}{
						"hostname": fmt.Sprintf("www%d.google.com", endpointCount),
						"timeout":  5,
					},
					HealthSettings: &m.CheckHealthSettings{
						NumProbes: 1,
						Steps:     3,
					},
				},
			},
		}
		err := AddEndpoint(e)
		So(err, ShouldBeNil)
		endpointCount++
		So(e.Id, ShouldNotEqual, 0)
		So(e.Slug, ShouldEqual, fmt.Sprintf("www%d_google_com", endpointCount-1))
		So(e.Created.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
		So(e.Created.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
		So(e.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
		So(e.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
		So(len(e.Checks), ShouldEqual, 2)
		for _, c := range e.Checks {
			So(c.Id, ShouldNotEqual, 0)
			So(c.EndpointId, ShouldEqual, e.Id)
			So(c.Created.Unix(), ShouldEqual, e.Created.Unix())
			So(c.Updated.Unix(), ShouldEqual, e.Updated.Unix())
			So(c.Frequency, ShouldEqual, 60)
			So(c.Offset, ShouldEqual, e.Id%c.Frequency)
			switch c.Type {
			case m.PING_CHECK:
				So(len(c.Settings), ShouldEqual, 2)
				Convey(fmt.Sprintf("probe list for %s check should be updated", c.Type), func() {
					probes, err := GetProbesForCheck(&c)
					So(err, ShouldBeNil)
					So(len(probes), ShouldEqual, 3)
					So(probes, ShouldContain, int64(1))
					So(probes, ShouldContain, int64(2))
					So(probes, ShouldContain, int64(4))
				})
			case m.HTTP_CHECK:
				So(len(c.Settings), ShouldEqual, 5)
				Convey(fmt.Sprintf("probe list for %s check should be updated", c.Type), func() {
					probes, err := GetProbesForCheck(&c)
					So(err, ShouldBeNil)
					So(len(probes), ShouldEqual, 5)
				})
			default:
				t.Fatal("unknown check exists.")
			}
		}

		Convey("When replacing endpoint tags", func() {
			e.Tags = []string{"foo", "bar"}
			err := UpdateEndpoint(e)
			So(err, ShouldBeNil)
			So(e.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, e.Created.Unix())
			So(e.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
			Convey("Tags should be updated in DB", func() {
				updated, err := GetEndpointById(e.OrgId, e.Id)
				So(err, ShouldBeNil)
				So(len(updated.Tags), ShouldEqual, 2)
				So(updated.Tags, ShouldContain, "foo")
				So(updated.Tags, ShouldContain, "bar")
			})

		})
		Convey("When removing endpoint tag", func() {
			e.Tags = []string{"test"}
			err := UpdateEndpoint(e)
			So(err, ShouldBeNil)
			So(e.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, e.Created.Unix())
			So(e.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
			Convey("Tags should be updated in DB", func() {
				updated, err := GetEndpointById(e.OrgId, e.Id)
				So(err, ShouldBeNil)
				So(len(updated.Tags), ShouldEqual, 1)
				So(updated.Tags, ShouldContain, "test")
			})
		})
		Convey("When adding additional endpoint tags", func() {
			e.Tags = []string{"test", "dev", "foo"}
			err := UpdateEndpoint(e)
			So(err, ShouldBeNil)
			So(e.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, e.Created.Unix())
			So(e.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
			Convey("Tags should be updated in DB", func() {
				updated, err := GetEndpointById(e.OrgId, e.Id)
				So(err, ShouldBeNil)
				So(len(updated.Tags), ShouldEqual, 3)
				So(updated.Tags, ShouldContain, "foo")
				So(updated.Tags, ShouldContain, "dev")
				So(updated.Tags, ShouldContain, "test")
			})
		})
		Convey("When updating endpoint checks", func() {
			e.Checks[0].Route = &m.CheckRoute{
				Type: m.RouteByIds,
				Config: map[string]interface{}{
					"ids": []int64{1},
				},
			}
			err := UpdateEndpoint(e)
			So(err, ShouldBeNil)
			So(e.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, e.Created.Unix())
			So(e.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
			for _, c := range e.Checks {
				So(c.Id, ShouldNotEqual, 0)
				So(c.EndpointId, ShouldEqual, e.Id)
				So(c.Created.Unix(), ShouldEqual, e.Created.Unix())
				So(c.Updated.Unix(), ShouldEqual, e.Updated.Unix())
				So(c.Frequency, ShouldEqual, 60)
				So(c.Offset, ShouldEqual, e.Id%c.Frequency)
				switch c.Type {
				case m.PING_CHECK:
					So(len(c.Settings), ShouldEqual, 2)
					Convey(fmt.Sprintf("probe list for %s check should be updated", c.Type), func() {
						probes, err := GetProbesForCheck(&c)
						So(err, ShouldBeNil)
						So(len(probes), ShouldEqual, 3)
						So(probes, ShouldContain, int64(1))
						So(probes, ShouldContain, int64(2))
						So(probes, ShouldContain, int64(4))
					})
				case m.HTTP_CHECK:
					So(len(c.Settings), ShouldEqual, 5)
					Convey(fmt.Sprintf("probe list for %s check should be updated", c.Type), func() {
						probes, err := GetProbesForCheck(&c)
						So(err, ShouldBeNil)
						So(len(probes), ShouldEqual, 1)
						So(probes[0], ShouldEqual, int64(1))
					})
				default:
					t.Fatal("unknown check exists.")
				}
			}

		})

	})
	Convey("When getting checks for probe", t, func() {
		checks, err := GetProbeChecksWithEndpointSlug(&m.ProbeDTO{Id: 4})
		So(err, ShouldBeNil)
		// 2 checks for each endpoint created.  But then we updated 2 endpoints
		// end changed their routing.
		So(len(checks), ShouldEqual, (endpointCount*2)-2)
	})
}
