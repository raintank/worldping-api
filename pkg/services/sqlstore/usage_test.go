package sqlstore

import (
	"fmt"
	"testing"

	m "github.com/raintank/worldping-api/pkg/models"
	. "github.com/smartystreets/goconvey/convey"
)

func TestUsageQuery(t *testing.T) {
	InitTestDB(t)
	err := AddProbe(&m.ProbeDTO{
		Name:      "public",
		OrgId:     1,
		Tags:      []string{"test"},
		Public:    false,
		Latitude:  1.0,
		Longitude: 1.0,
		Online:    false,
		Enabled:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int64{1, 2, 3, 4, 5} {
		err := AddProbe(&m.ProbeDTO{
			Name:      fmt.Sprintf("test%d", i),
			OrgId:     i % 2,
			Tags:      []string{"test"},
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
	for _, i := range []int64{1, 2, 3, 4, 5, 6} {
		e := &m.EndpointDTO{
			Name:  fmt.Sprintf("www%d.google.com", i),
			OrgId: i % 3,
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
						"host":    fmt.Sprintf("www%d.google.com", i),
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
						"hostname": fmt.Sprintf("www%d.google.com", i),
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
		if err != nil {
			t.Fatal(err)
		}
	}
	Convey("when getting usage metrics", t, func() {
		usage, err := GetUsage()
		So(err, ShouldBeNil)
		Convey("endpoint data should be accurate", func() {
			So(usage.Endpoints.Total, ShouldEqual, 6)
			So(len(usage.Endpoints.PerOrg), ShouldEqual, 3)
			So(usage.Endpoints.PerOrg["1"], ShouldEqual, 2)
		})
		Convey("probe data should be accurate", func() {
			So(usage.Probes.Total, ShouldEqual, 6)
			So(len(usage.Probes.PerOrg), ShouldEqual, 2)
			So(usage.Probes.PerOrg["1"], ShouldEqual, 4)
		})
		Convey("checks data should be accurate", func() {
			So(usage.Checks.Total, ShouldEqual, 12)
			So(usage.Checks.HTTP.Total, ShouldEqual, 6)
			So(usage.Checks.HTTPS.Total, ShouldEqual, 0)
			So(usage.Checks.PING.Total, ShouldEqual, 6)
			So(usage.Checks.DNS.Total, ShouldEqual, 0)
			So(usage.Endpoints.PerOrg["1"], ShouldEqual, 2)
			So(len(usage.Checks.HTTP.PerOrg), ShouldEqual, 3)
			So(len(usage.Checks.HTTPS.PerOrg), ShouldEqual, 0)
			So(len(usage.Checks.PING.PerOrg), ShouldEqual, 3)
			So(len(usage.Checks.DNS.PerOrg), ShouldEqual, 0)
			So(usage.Checks.HTTP.PerOrg["1"], ShouldEqual, 2)
		})

	})
}
