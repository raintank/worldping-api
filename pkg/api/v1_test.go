package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Unknwon/macaron"
	"github.com/go-xorm/xorm"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/services/sqlstore/sqlutil"
	"github.com/raintank/worldping-api/pkg/setting"
	. "github.com/smartystreets/goconvey/convey"
)

func InitTestDB(t *testing.T) {
	t.Log("InitTestDB")
	x, err := xorm.NewEngine(sqlutil.TestDB_Sqlite3.DriverName, sqlutil.TestDB_Sqlite3.ConnStr)
	//x, err := xorm.NewEngine(sqlutil.TestDB_Mysql.DriverName, sqlutil.TestDB_Mysql.ConnStr)
	//x, err := xorm.NewEngine(sqlutil.TestDB_Postgres.DriverName, sqlutil.TestDB_Postgres.ConnStr)
	x.SetMaxOpenConns(1)
	if err != nil {
		t.Fatalf("Failed to init in memory sqllite3 db %v", err)
	}

	sqlutil.CleanDB(x)

	if err := sqlstore.SetEngine(x, false); err != nil {
		t.Fatal(err)
	}
}

func addAuthHeader(req *http.Request) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", setting.AdminKey))
}

func addContentTypeHeader(req *http.Request) {
	req.Header.Add("Content-Type", "application/json")
}

func TestQuotasV1Api(t *testing.T) {
	InitTestDB(t)
	r := macaron.Classic()
	setting.AdminKey = "test"
	setting.Quota = setting.QuotaSettings{
		Enabled: false,
		Org: &setting.OrgQuota{
			Endpoint: 10,
			Probe:    10,
		},
		Global: &setting.GlobalQuota{
			Endpoint: -1,
			Probe:    -1,
		},
	}
	Register(r)

	Convey("When quotas not enabled", t, func() {
		setting.Quota.Enabled = false
		Convey("Given GET request for /api/org/quotas", func() {
			resp := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "/api/org/quotas", nil)
			So(err, ShouldBeNil)
			addAuthHeader(req)

			r.ServeHTTP(resp, req)
			Convey("should return 200", func() {
				So(resp.Code, ShouldEqual, 200)
				Convey("quota response should be valid", func() {
					quota := make([]m.OrgQuotaDTO, 0)
					err := json.Unmarshal(resp.Body.Bytes(), &quota)
					So(err, ShouldBeNil)

					So(len(quota), ShouldEqual, 2)
					So(quota[0].OrgId, ShouldEqual, 1)
					So(quota[0].Limit, ShouldEqual, -1)
					So(quota[0].Used, ShouldEqual, -10)
					for _, q := range quota {
						So(q.Target, ShouldBeIn, "endpoint", "probe")
					}
				})
			})
		})
	})

	endpointCount := 0
	probeCount := 0
	Convey("When quotas are enabled", t, func() {
		setting.Quota.Enabled = true
		Convey("Given GET request for /api/org/quotas", func() {
			resp := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "/api/org/quotas", nil)
			So(err, ShouldBeNil)
			addAuthHeader(req)

			r.ServeHTTP(resp, req)
			Convey("should return 200", func() {
				So(resp.Code, ShouldEqual, 200)
				Convey("quota response should be valid", func() {
					quota := make([]m.OrgQuotaDTO, 0)
					err := json.Unmarshal(resp.Body.Bytes(), &quota)
					So(err, ShouldBeNil)

					So(len(quota), ShouldEqual, 2)

					for i := range []int{1, 2, 3} {
						Convey(fmt.Sprintf("when %d endpoints", i), func() {
							err := sqlstore.AddEndpoint(&m.EndpointDTO{
								Name:  fmt.Sprintf("test%d", i),
								OrgId: 1,
							})
							endpointCount = i
							So(err, ShouldBeNil)
							for _, q := range quota {
								So(quota[0].OrgId, ShouldEqual, 1)
								So(quota[0].Limit, ShouldEqual, 10)
								So(q.Target, ShouldBeIn, "endpoint", "probe")
								if q.Target == "endpoint" {
									So(q.Used, ShouldEqual, endpointCount)
								}
							}
						})
					}
					for i := range []int{1, 2, 3} {
						Convey(fmt.Sprintf("when %d probes", i), func() {
							err := sqlstore.AddProbe(&m.ProbeDTO{
								Name:  fmt.Sprintf("test%d", i),
								OrgId: 1,
							})
							probeCount = i
							So(err, ShouldBeNil)
							for _, q := range quota {
								So(quota[0].OrgId, ShouldEqual, 1)
								So(quota[0].Limit, ShouldEqual, 10)
								So(q.Target, ShouldBeIn, "endpoint", "probe")
								if q.Target == "probe" {
									So(q.Used, ShouldEqual, probeCount)
								}
							}
						})
					}
				})
			})
		})
	})
}

func TestMonitorTypesV1Api(t *testing.T) {
	r := macaron.Classic()
	setting.AdminKey = "test"
	Register(r)

	Convey("Given GET request for /api/monitor_types", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/monitor_types", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("monitor_types response should be valid", func() {
				monitorTypes := make([]m.MonitorTypeDTO, 0)
				err := json.Unmarshal(resp.Body.Bytes(), &monitorTypes)
				So(err, ShouldBeNil)
				So(len(monitorTypes), ShouldEqual, 4)
				for _, mType := range monitorTypes {
					So(mType.Name, ShouldBeIn, "HTTP", "HTTPS", "Ping", "DNS")
					switch mType.Name {
					case "HTTP":
						So(len(mType.Settings), ShouldEqual, 7)
					case "HTTPS":
						So(len(mType.Settings), ShouldEqual, 8)
					case "Ping":
						So(len(mType.Settings), ShouldEqual, 2)
					case "DNS":
						So(len(mType.Settings), ShouldEqual, 6)
					}
				}
			})
		})
	})
}

func populateCollectors(t *testing.T) {
	for _, i := range []int{1, 2, 3} {
		err := sqlstore.AddProbe(&m.ProbeDTO{
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
		err := sqlstore.AddProbe(pubProbe)
		if err != nil {
			t.Fatal(err)
		}

		// add tags for orgid 1 for pub probes owned by orgId2
		pubProbe.OrgId = 1
		pubProbe.Tags = []string{"pTest", "test", "foo"}
		err = sqlstore.UpdateProbe(pubProbe)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestProbesV1Api(t *testing.T) {
	InitTestDB(t)
	r := macaron.Classic()
	setting.AdminKey = "test"
	setting.Quota = setting.QuotaSettings{
		Enabled: true,
		Org: &setting.OrgQuota{
			Endpoint: 4,
			Probe:    4,
		},
		Global: &setting.GlobalQuota{
			Endpoint: -1,
			Probe:    -1,
		},
	}
	Register(r)
	populateCollectors(t)

	Convey("Given GET request for /api/collectors", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/collectors", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("collectors response should be valid", func() {
				probes := make([]m.ProbeDTO, 0)
				err := json.Unmarshal(resp.Body.Bytes(), &probes)
				So(err, ShouldBeNil)
				So(len(probes), ShouldEqual, 5)
				for _, probe := range probes {
					if probe.Public {
						So(len(probe.Tags), ShouldEqual, 3)
						So(probe.OrgId, ShouldEqual, 2)
						So(probe.Name, ShouldStartWith, "public")
					} else {
						So(len(probe.Tags), ShouldEqual, 2)
						So(probe.OrgId, ShouldEqual, 1)
						So(probe.Name, ShouldStartWith, "test")
					}
				}
			})
		})
	})
	Convey("Given GET request for /api/collectors/1", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/collectors/1", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("collectors response should be valid", func() {
				probe := m.ProbeDTO{}
				err := json.Unmarshal(resp.Body.Bytes(), &probe)
				So(err, ShouldBeNil)
				So(probe.Name, ShouldEqual, "test1")
				So(probe.Public, ShouldEqual, false)
				So(len(probe.Tags), ShouldEqual, 2)
				So(probe.Tags, ShouldContain, "test")
				So(probe.Tags, ShouldContain, "dev1")
			})
		})
	})
	Convey("Given GET request for /api/collectors/locations", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/collectors/locations", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("collector locations response should be valid", func() {
				locations := make([]m.ProbeLocationDTO, 0)
				err := json.Unmarshal(resp.Body.Bytes(), &locations)
				So(err, ShouldBeNil)
				So(len(locations), ShouldEqual, 5)
				So(locations[0].Latitude, ShouldEqual, 1.0)
			})
		})
	})
	Convey("Given POST request to update private collector", t, func() {
		resp := httptest.NewRecorder()
		payload, err := json.Marshal(&m.ProbeDTO{
			Id:        1,
			Name:      "nameChange1",
			OrgId:     1,
			Tags:      []string{"test", "foo", "bar"},
			Public:    false,
			Latitude:  1.0,
			Longitude: 1.0,
			Online:    false,
			Enabled:   false,
		})
		So(err, ShouldBeNil)
		req, err := http.NewRequest("POST", "/api/collectors", bytes.NewReader(payload))
		So(err, ShouldBeNil)
		addAuthHeader(req)
		addContentTypeHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("entry in the DB should be updated", func() {
				probe, err := sqlstore.GetProbeById(1, 1)
				So(err, ShouldBeNil)

				So(probe.Name, ShouldEqual, "nameChange1")
				So(probe.Public, ShouldEqual, false)
				So(len(probe.Tags), ShouldEqual, 3)
				So(probe.Tags, ShouldContain, "test")
				So(probe.Tags, ShouldContain, "foo")
				So(probe.Tags, ShouldContain, "bar")
				So(probe.Enabled, ShouldEqual, false)
			})
		})
	})
	Convey("Given PUT request to create private collector", t, func() {
		resp := httptest.NewRecorder()
		pre := time.Now()
		payload, err := json.Marshal(&m.ProbeDTO{
			Name:      "test4",
			OrgId:     1,
			Tags:      []string{"test", "foo", "bar"},
			Public:    false,
			Latitude:  1.0,
			Longitude: 1.0,
			Online:    false,
			Enabled:   false,
		})
		So(err, ShouldBeNil)
		req, err := http.NewRequest("PUT", "/api/collectors", bytes.NewReader(payload))
		So(err, ShouldBeNil)
		addAuthHeader(req)
		addContentTypeHeader(req)

		r.ServeHTTP(resp, req)
		Convey("response should be 200 with probeDTO", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("entry in the DB should be created", func() {
				probe, err := sqlstore.GetProbeByName("test4", 1)
				So(err, ShouldBeNil)
				So(probe.OrgId, ShouldEqual, 1)
				So(probe.Name, ShouldEqual, "test4")
				So(probe.Public, ShouldEqual, false)
				So(len(probe.Tags), ShouldEqual, 3)
				So(probe.Tags, ShouldContain, "test")
				So(probe.Tags, ShouldContain, "foo")
				So(probe.Tags, ShouldContain, "bar")
				So(probe.Enabled, ShouldEqual, false)
				So(probe.Created.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
				So(probe.Created.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
				So(probe.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
				So(probe.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
				So(probe.OnlineChange.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
				So(probe.OnlineChange.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
				So(probe.EnabledChange.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
				So(probe.EnabledChange.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
				Convey("resp should match what is in the db", func() {
					probeResp := m.ProbeDTO{}
					err := json.Unmarshal(resp.Body.Bytes(), &probeResp)

					So(err, ShouldBeNil)
					So(probeResp.Id, ShouldEqual, probe.Id)
					So(probeResp.Name, ShouldEqual, "test4")
					So(probeResp.Slug, ShouldEqual, "test4")
					So(probeResp.Public, ShouldEqual, false)
					So(len(probeResp.Tags), ShouldEqual, 3)
					So(probeResp.Tags, ShouldContain, "test")
					So(probeResp.Tags, ShouldContain, "foo")
					So(probeResp.Tags, ShouldContain, "bar")
					So(probeResp.Created.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
					So(probeResp.Created.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
					So(probeResp.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
					So(probeResp.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
					So(probeResp.OnlineChange.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
					So(probeResp.OnlineChange.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
					So(probeResp.EnabledChange.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
					So(probeResp.EnabledChange.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
				})
			})
		})
		Convey("response should fail due to quota", func() {
			So(resp.Code, ShouldEqual, 403)
		})
	})
}

func populateEndpoints(t *testing.T) {
	for _, i := range []int{1, 2, 3} {
		err := sqlstore.AddEndpoint(&m.EndpointDTO{
			Name:  fmt.Sprintf("www%d.google.com", i),
			OrgId: 1,
			Tags:  []string{"test", fmt.Sprintf("dev%d", i%2)},
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
							"ids": []int64{1, 2, 4},
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
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, i := range []int{1, 2, 3} {
		err := sqlstore.AddEndpoint(&m.EndpointDTO{
			Name:  fmt.Sprintf("www%d.google.com", i),
			OrgId: 2,
			Tags:  []string{"test2", fmt.Sprintf("2dev%d", i%2)},
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
							"ids": []int64{1, 2, 4},
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
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestEndpointV1Api(t *testing.T) {
	InitTestDB(t)
	r := macaron.Classic()
	setting.AdminKey = "test"
	setting.Quota = setting.QuotaSettings{
		Enabled: true,
		Org: &setting.OrgQuota{
			Endpoint: 4,
			Probe:    4,
		},
		Global: &setting.GlobalQuota{
			Endpoint: -1,
			Probe:    -1,
		},
	}
	Register(r)
	populateCollectors(t)
	populateEndpoints(t)

	Convey("Given GET request for /api/endpoints", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/endpoints", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("endpoints response should be valid", func() {
				endpoints := make([]m.EndpointDTO, 0)
				err := json.Unmarshal(resp.Body.Bytes(), &endpoints)
				So(err, ShouldBeNil)
				So(len(endpoints), ShouldEqual, 3)
				for _, endpoint := range endpoints {
					So(len(endpoint.Tags), ShouldEqual, 2)
					So(endpoint.OrgId, ShouldEqual, 1)
					So(endpoint.Name, ShouldEndWith, "google.com")
					So(endpoint.Slug, ShouldEndWith, "google_com")
				}
			})
		})
	})
	Convey("Given GET request for /api/endpoints/1", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/endpoints/1", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("endpoints response should be valid", func() {
				endpoint := m.EndpointDTO{}
				err := json.Unmarshal(resp.Body.Bytes(), &endpoint)
				So(err, ShouldBeNil)
				So(endpoint.Name, ShouldEqual, "www1.google.com")
				So(endpoint.Slug, ShouldEqual, "www1_google_com")
				So(endpoint.OrgId, ShouldEqual, 1)
				So(len(endpoint.Tags), ShouldEqual, 2)
				So(endpoint.Tags, ShouldContain, "test")
				So(endpoint.Tags, ShouldContain, "dev1")
			})
		})
	})

	Convey("Given POST request to update endpoint", t, func() {
		resp := httptest.NewRecorder()
		payload, err := json.Marshal(&m.UpdateEndpointCommand{
			Id:   1,
			Name: "www1.google.com",
			Tags: []string{"test", "foo", "bar"},
		})
		So(err, ShouldBeNil)
		req, err := http.NewRequest("POST", "/api/endpoints", bytes.NewReader(payload))
		So(err, ShouldBeNil)
		addAuthHeader(req)
		addContentTypeHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("entry in the DB should be updated", func() {
				endpoint, err := sqlstore.GetEndpointById(1, 1)
				So(err, ShouldBeNil)

				So(endpoint.Name, ShouldEqual, "www1.google.com")
				So(endpoint.Slug, ShouldEqual, "www1_google_com")
				So(endpoint.OrgId, ShouldEqual, 1)
				So(len(endpoint.Tags), ShouldEqual, 3)
				So(endpoint.Tags, ShouldContain, "test")
				So(endpoint.Tags, ShouldContain, "foo")
				So(endpoint.Tags, ShouldContain, "bar")

				// for V1 api, endpoint updates can only update name and tags.
				// checks should be ignored in this request.
				So(len(endpoint.Checks), ShouldEqual, 2)
			})
		})
	})
	Convey("Given PUT request to create endpoint", t, func() {
		resp := httptest.NewRecorder()
		pre := time.Now()
		payload, err := json.Marshal(&m.AddEndpointCommand{
			Name:  "www6.google.com",
			OrgId: 1,
			Tags:  []string{"test", "foo", "bar"},
			Monitors: []*m.AddMonitorCommand{
				{
					EndpointId:    -1,
					Frequency:     60,
					MonitorTypeId: 1,
					Enabled:       true,
					CollectorTags: []string{"test"},
					Settings: []m.MonitorSettingDTO{
						{Variable: "host", Value: "www6.google.com"},
						{"path", "/foo"},
						{"port", "80"},
						{"method", "GET"},
						{"timeout", "5"},
					},
					HealthSettings: &m.CheckHealthSettings{
						NumProbes: 1,
						Steps:     3,
					},
				},
				{
					EndpointId:    -1,
					Frequency:     60,
					MonitorTypeId: 3,
					Enabled:       true,
					CollectorIds:  []int64{1, 2, 4},
					Settings: []m.MonitorSettingDTO{
						{"hostname", "www6.google.com"},
						{"timeout", "5"},
					},
					HealthSettings: &m.CheckHealthSettings{
						NumProbes: 1,
						Steps:     3,
					},
				},
			},
		})

		So(err, ShouldBeNil)
		req, err := http.NewRequest("PUT", "/api/endpoints", bytes.NewReader(payload))
		So(err, ShouldBeNil)
		addAuthHeader(req)
		addContentTypeHeader(req)

		r.ServeHTTP(resp, req)
		Convey("response should be 200 with endpointDTO", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("resp should match what is in the db", func() {
				endpointResp := m.EndpointDTO{}
				err := json.Unmarshal(resp.Body.Bytes(), &endpointResp)

				So(err, ShouldBeNil)
				So(endpointResp.Id, ShouldNotEqual, 0)
				So(endpointResp.Name, ShouldEqual, "www6.google.com")
				So(endpointResp.Slug, ShouldEqual, "www6_google_com")
				So(len(endpointResp.Tags), ShouldEqual, 3)
				So(endpointResp.Tags, ShouldContain, "test")
				So(endpointResp.Tags, ShouldContain, "foo")
				So(endpointResp.Tags, ShouldContain, "bar")
				So(endpointResp.Created.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
				So(endpointResp.Created.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
				So(endpointResp.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
				So(endpointResp.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
				Convey("entry in the DB should be created", func() {
					endpoint, err := sqlstore.GetEndpointById(1, endpointResp.Id)
					So(err, ShouldBeNil)
					So(endpoint, ShouldNotBeNil)
					So(endpoint.OrgId, ShouldEqual, 1)
					So(endpoint.Name, ShouldEqual, "www6.google.com")
					So(endpoint.Slug, ShouldEqual, "www6_google_com")
					So(len(endpoint.Tags), ShouldEqual, 3)
					So(endpoint.Tags, ShouldContain, "test")
					So(endpoint.Tags, ShouldContain, "foo")
					So(endpoint.Tags, ShouldContain, "bar")
					So(endpoint.Created.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
					So(endpoint.Created.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
					So(endpoint.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
					So(endpoint.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())

					So(len(endpoint.Checks), ShouldEqual, 2)
					Convey("Checks should be valid", func() {
						for _, c := range endpoint.Checks {
							So(c.Type, ShouldBeIn, m.HTTP_CHECK, m.PING_CHECK)
							So(c.Frequency, ShouldEqual, 60)
							So(c.Enabled, ShouldEqual, true)
							So(c.EndpointId, ShouldEqual, endpoint.Id)
							So(c.OrgId, ShouldEqual, endpoint.OrgId)
							switch c.Type {
							case m.HTTP_CHECK:
								So(c.Route.Type, ShouldEqual, m.RouteByTags)
								So(len(c.Route.Config["tags"].([]string)), ShouldEqual, 1)
								So(len(c.Settings), ShouldEqual, 5)
								probes, err := sqlstore.GetProbesForCheck(&c)
								So(err, ShouldBeNil)
								So(len(probes), ShouldEqual, 5)

							case m.PING_CHECK:
								So(c.Route.Type, ShouldEqual, m.RouteByIds)
								So(len(c.Route.Config["ids"].([]int64)), ShouldEqual, 3)
								So(len(c.Settings), ShouldEqual, 2)

								probes, err := sqlstore.GetProbesForCheck(&c)
								So(err, ShouldBeNil)
								So(len(probes), ShouldEqual, 3)

							}
						}
					})

				})
			})

		})
		Convey("response should fail due to quota", func() {
			So(resp.Code, ShouldEqual, 403)
		})
	})
}

func TestMonitorV1Api(t *testing.T) {
	InitTestDB(t)
	r := macaron.Classic()
	setting.AdminKey = "test"
	setting.Quota = setting.QuotaSettings{
		Enabled: true,
		Org: &setting.OrgQuota{
			Endpoint: 4,
			Probe:    4,
		},
		Global: &setting.GlobalQuota{
			Endpoint: -1,
			Probe:    -1,
		},
	}
	Register(r)
	populateCollectors(t)
	populateEndpoints(t)

	Convey("Given GET request for /api/monitors?endpoint_id=1", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/monitors?endpoint_id=1", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("endpoints response should be valid", func() {
				monitors := make([]m.MonitorDTO, 0)
				err := json.Unmarshal(resp.Body.Bytes(), &monitors)
				So(err, ShouldBeNil)
				So(len(monitors), ShouldEqual, 2)
				for _, monitor := range monitors {
					So(monitor.OrgId, ShouldEqual, 1)
					So(monitor.EndpointId, ShouldEqual, 1)
					So(monitor.EndpointSlug, ShouldEqual, "www1_google_com")
					So(monitor.MonitorTypeName, ShouldBeIn, "http", "ping")
					switch monitor.MonitorTypeName {
					case "http":
						So(len(monitor.Collectors), ShouldEqual, 5)
						So(monitor.MonitorTypeId, ShouldEqual, 1)
						So(len(monitor.CollectorTags), ShouldEqual, 1)
						So(len(monitor.CollectorIds), ShouldEqual, 0)
						So(len(monitor.Settings), ShouldEqual, 5)
					case "ping":
						So(len(monitor.Collectors), ShouldEqual, 3)
						So(len(monitor.CollectorIds), ShouldEqual, 3)
						So(len(monitor.CollectorTags), ShouldEqual, 0)
						So(monitor.MonitorTypeId, ShouldEqual, 3)
						So(len(monitor.Settings), ShouldEqual, 2)
					}
				}
			})
		})
	})
	Convey("Given POST request to update monitor", t, func() {
		resp := httptest.NewRecorder()
		pre := time.Now()
		payload, err := json.Marshal(&m.UpdateMonitorCommand{
			EndpointId:    1,
			Id:            1,
			Frequency:     60,
			MonitorTypeId: 1,
			Enabled:       true,
			CollectorTags: []string{},
			CollectorIds:  []int64{1, 5},
			Settings: []m.MonitorSettingDTO{
				{Variable: "host", Value: "www1.google.com"},
				{"path", "/foo"},
				{"port", "8080"},
				{"method", "GET"},
				{"timeout", "5"},
			},
			HealthSettings: &m.CheckHealthSettings{
				NumProbes: 1,
				Steps:     3,
			},
		})
		So(err, ShouldBeNil)
		req, err := http.NewRequest("POST", "/api/monitors", bytes.NewReader(payload))
		So(err, ShouldBeNil)
		addAuthHeader(req)
		addContentTypeHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("entry in the DB should be updated", func() {
				check, err := sqlstore.GetCheckById(1, 1)
				So(err, ShouldBeNil)

				So(check.EndpointId, ShouldEqual, 1)
				So(check.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
				So(check.Route.Type, ShouldEqual, m.RouteByIds)
				So(len(check.Settings), ShouldEqual, 5)
				So(check.Settings, ShouldContainKey, "port")
				So(check.Settings["port"], ShouldEqual, 8080)
			})
		})
	})
	Convey("Given PUT request to create monitor", t, func() {
		resp := httptest.NewRecorder()
		pre := time.Now()
		payload, err := json.Marshal(&m.AddMonitorCommand{
			EndpointId:    1,
			Frequency:     60,
			MonitorTypeId: 2,
			Enabled:       true,
			CollectorTags: []string{},
			CollectorIds:  []int64{1, 5},
			Settings: []m.MonitorSettingDTO{
				{Variable: "host", Value: "www7.google.com"},
				{"path", "/foo"},
				{"port", "8080"},
				{"method", "GET"},
				{"timeout", "5"},
			},
			HealthSettings: &m.CheckHealthSettings{
				NumProbes: 1,
				Steps:     3,
			},
		})
		So(err, ShouldBeNil)
		req, err := http.NewRequest("PUT", "/api/monitors", bytes.NewReader(payload))
		So(err, ShouldBeNil)
		addAuthHeader(req)
		addContentTypeHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200 with MonitorDTO", func() {
			So(resp.Code, ShouldEqual, 200)
			monResp := m.MonitorDTO{}
			err := json.Unmarshal(resp.Body.Bytes(), &monResp)
			So(err, ShouldBeNil)

			So(monResp.Id, ShouldNotEqual, 0)
			So(len(monResp.Collectors), ShouldEqual, 2)

			Convey("entry in the DB should be updated", func() {
				check, err := sqlstore.GetCheckById(1, monResp.Id)
				So(err, ShouldBeNil)

				So(check.Id, ShouldEqual, monResp.Id)
				So(check.EndpointId, ShouldEqual, 1)
				So(check.Created.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
				So(check.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
				So(check.Route.Type, ShouldEqual, m.RouteByIds)
				So(len(check.Settings), ShouldEqual, 5)
				So(check.Settings, ShouldContainKey, "port")
				So(check.Settings["port"], ShouldEqual, 8080)
			})
		})
	})
}
