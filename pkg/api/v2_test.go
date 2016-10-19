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
	"github.com/raintank/worldping-api/pkg/api/rbody"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
	. "github.com/smartystreets/goconvey/convey"
)

func TestQuotasV2Api(t *testing.T) {
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
		Convey("Given GET request for /api/v2/quotas", func() {
			resp := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "/api/v2/quotas", nil)
			So(err, ShouldBeNil)
			addAuthHeader(req)

			r.ServeHTTP(resp, req)
			Convey("should return 200", func() {
				So(resp.Code, ShouldEqual, 200)
				Convey("quota response should be valid ApiResponse", func() {
					response := rbody.ApiResponse{}
					err := json.Unmarshal(resp.Body.Bytes(), &response)
					So(err, ShouldBeNil)
					So(response.Meta.Code, ShouldEqual, 200)
					So(response.Meta.Type, ShouldEqual, "quotas")

					quota := make([]m.OrgQuotaDTO, 0)
					err = json.Unmarshal(response.Body, &quota)
					So(err, ShouldBeNil)

					So(len(quota), ShouldEqual, 2)
					So(quota[0].OrgId, ShouldEqual, 1)
					So(quota[0].Limit, ShouldEqual, -1)
					So(quota[0].Used, ShouldEqual, -1)
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
			req, err := http.NewRequest("GET", "/api/v2/quotas", nil)
			So(err, ShouldBeNil)
			addAuthHeader(req)

			r.ServeHTTP(resp, req)
			Convey("should return 200", func() {
				So(resp.Code, ShouldEqual, 200)
				Convey("quota response should be valid", func() {
					response := rbody.ApiResponse{}
					err := json.Unmarshal(resp.Body.Bytes(), &response)
					So(err, ShouldBeNil)
					So(response.Meta.Code, ShouldEqual, 200)
					So(response.Meta.Type, ShouldEqual, "quotas")
					quota := make([]m.OrgQuotaDTO, 0)
					err = json.Unmarshal(response.Body, &quota)
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

func TestProbesV2Api(t *testing.T) {
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

	Convey("Given GET request for /api/v2/probes", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/v2/probes", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("probes response should be valid", func() {
				response := rbody.ApiResponse{}
				err := json.Unmarshal(resp.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Meta.Code, ShouldEqual, 200)
				So(response.Meta.Type, ShouldEqual, "probes")
				probes := make([]m.ProbeDTO, 0)
				err = json.Unmarshal(response.Body, &probes)
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
	Convey("Given GET request for /api/v2/probes/1", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/v2/probes/1", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("collectors response should be valid", func() {
				response := rbody.ApiResponse{}
				err := json.Unmarshal(resp.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Meta.Code, ShouldEqual, 200)
				So(response.Meta.Type, ShouldEqual, "probe")
				probe := m.ProbeDTO{}
				err = json.Unmarshal(response.Body, &probe)
				So(err, ShouldBeNil)
				So(probe.Name, ShouldEqual, "test1")
				So(probe.Public, ShouldEqual, false)
				So(len(probe.Tags), ShouldEqual, 2)
				So(probe.Tags, ShouldContain, "test")
				So(probe.Tags, ShouldContain, "dev1")
			})
		})
	})
	Convey("Given GET request for /api/v2/probes/locations", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/v2/probes/locations", nil)
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
	Convey("Given PUT request to update private probe", t, func() {
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
		req, err := http.NewRequest("PUT", "/api/v2/probes", bytes.NewReader(payload))
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
	Convey("Given PUT request to add tags to public probe", t, func() {
		resp := httptest.NewRecorder()
		payload, err := json.Marshal(&m.ProbeDTO{
			Id:        4,
			Name:      "nameChange1",
			OrgId:     2,
			Tags:      []string{"test", "foo", "bar"},
			Public:    true,
			Latitude:  1.0,
			Longitude: 1.0,
			Online:    false,
			Enabled:   false,
		})
		So(err, ShouldBeNil)
		req, err := http.NewRequest("PUT", "/api/v2/probes", bytes.NewReader(payload))
		So(err, ShouldBeNil)
		addAuthHeader(req)
		addContentTypeHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("entry in the DB should be updated", func() {
				probe, err := sqlstore.GetProbeById(4, 1)
				So(err, ShouldBeNil)

				So(probe.Name, ShouldEqual, "public1")
				So(probe.Public, ShouldEqual, true)
				So(len(probe.Tags), ShouldEqual, 3)
				So(probe.Tags, ShouldContain, "test")
				So(probe.Tags, ShouldContain, "foo")
				So(probe.Tags, ShouldContain, "bar")
				So(probe.Enabled, ShouldEqual, true)
			})
		})
	})
	Convey("Given POST request to create private probe", t, func() {
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
		req, err := http.NewRequest("POST", "/api/v2/probes", bytes.NewReader(payload))
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
					response := rbody.ApiResponse{}
					err := json.Unmarshal(resp.Body.Bytes(), &response)
					So(err, ShouldBeNil)
					So(response.Meta.Code, ShouldEqual, 200)
					So(response.Meta.Type, ShouldEqual, "probe")
					probeResp := m.ProbeDTO{}
					err = json.Unmarshal(response.Body, &probeResp)
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

func TestEndpointV2Api(t *testing.T) {
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

	Convey("Given GET request for /api/v2/endpoints", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/v2/endpoints", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("endpoints response should be valid", func() {
				response := rbody.ApiResponse{}
				err := json.Unmarshal(resp.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Meta.Code, ShouldEqual, 200)
				So(response.Meta.Type, ShouldEqual, "endpoints")
				endpoints := make([]m.EndpointDTO, 0)
				err = json.Unmarshal(response.Body, &endpoints)
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
	Convey("Given GET request for /api/v2/endpoints/1", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/v2/endpoints/1", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("endpoints response should be valid", func() {
				response := rbody.ApiResponse{}
				err := json.Unmarshal(resp.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Meta.Code, ShouldEqual, 200)
				So(response.Meta.Type, ShouldEqual, "endpoint")
				endpoint := m.EndpointDTO{}
				err = json.Unmarshal(response.Body, &endpoint)
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

	Convey("Given PUT request to update endpoint", t, func() {
		resp := httptest.NewRecorder()
		payload, err := json.Marshal(&m.EndpointDTO{
			Id:    1,
			Name:  "www1.google.com",
			OrgId: 1,
			Tags:  []string{"test", "foo", "bar"},
			Checks: []m.Check{
				{
					Id: 1,
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
						"host":    "www1.google.com",
						"path":    "/test",
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
						Type: m.RouteByTags,
						Config: map[string]interface{}{
							"tags": []string{"test"},
						},
					},
					Frequency: 60,
					Type:      m.HTTPS_CHECK,
					Enabled:   true,
					Settings: map[string]interface{}{
						"host":    "www1.google.com",
						"path":    "/",
						"port":    443,
						"method":  "GET",
						"timeout": 5,
					},
					HealthSettings: &m.CheckHealthSettings{
						NumProbes: 1,
						Steps:     3,
					},
				},
				{
					Id: 2,
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
						"hostname": "www1.google.com",
						"timeout":  5,
					},
					HealthSettings: &m.CheckHealthSettings{
						NumProbes: 1,
						Steps:     3,
					},
				},
			},
		})
		So(err, ShouldBeNil)
		req, err := http.NewRequest("PUT", "/api/v2/endpoints", bytes.NewReader(payload))
		So(err, ShouldBeNil)
		addAuthHeader(req)
		addContentTypeHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			response := rbody.ApiResponse{}
			err := json.Unmarshal(resp.Body.Bytes(), &response)
			So(err, ShouldBeNil)
			So(response.Meta.Code, ShouldEqual, 200)
			So(response.Meta.Type, ShouldEqual, "endpoint")
			endpointResp := m.EndpointDTO{}
			err = json.Unmarshal(response.Body, &endpointResp)
			So(err, ShouldBeNil)

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

				So(len(endpoint.Checks), ShouldEqual, 3)
				for _, check := range endpoint.Checks {
					switch check.Type {
					case m.HTTP_CHECK:
						So(check.Settings["path"], ShouldEqual, "/test")
					case m.HTTPS_CHECK:
						So(check.Settings["path"], ShouldEqual, "/")
					case m.PING_CHECK:
						So(len(check.Settings), ShouldEqual, 2)
					}
				}
			})
		})
	})
	Convey("Given POST request to create endpoint", t, func() {
		resp := httptest.NewRecorder()
		pre := time.Now()
		payload, err := json.Marshal(&m.EndpointDTO{
			Name:  "www6.google.com",
			OrgId: 1,
			Tags:  []string{"foo", "bar"},
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
						"host":    "www6.google.com",
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
						Type: m.RouteByTags,
						Config: map[string]interface{}{
							"tags": []string{"test"},
						},
					},
					Frequency: 60,
					Type:      m.HTTPS_CHECK,
					Enabled:   true,
					Settings: map[string]interface{}{
						"host":    "www6.google.com",
						"path":    "/",
						"port":    443,
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
						"hostname": "www6.google.com",
						"timeout":  5,
					},
					HealthSettings: &m.CheckHealthSettings{
						NumProbes: 1,
						Steps:     3,
					},
				},
			},
		})

		So(err, ShouldBeNil)
		req, err := http.NewRequest("POST", "/api/v2/endpoints", bytes.NewReader(payload))
		So(err, ShouldBeNil)
		addAuthHeader(req)
		addContentTypeHeader(req)

		r.ServeHTTP(resp, req)
		Convey("response should be 200 with endpointDTO", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("resp should match what is in the db", func() {
				response := rbody.ApiResponse{}
				err := json.Unmarshal(resp.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Meta.Code, ShouldEqual, 200)
				So(response.Meta.Type, ShouldEqual, "endpoint")
				endpointResp := m.EndpointDTO{}
				err = json.Unmarshal(response.Body, &endpointResp)
				So(err, ShouldBeNil)

				So(endpointResp.Id, ShouldNotEqual, 0)
				So(endpointResp.Name, ShouldEqual, "www6.google.com")
				So(endpointResp.Slug, ShouldEqual, "www6_google_com")
				So(len(endpointResp.Tags), ShouldEqual, 2)
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
					So(len(endpoint.Tags), ShouldEqual, 2)
					So(endpoint.Tags, ShouldContain, "foo")
					So(endpoint.Tags, ShouldContain, "bar")
					So(endpoint.Created.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
					So(endpoint.Created.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
					So(endpoint.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
					So(endpoint.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())

					So(len(endpoint.Checks), ShouldEqual, 3)
					Convey("Checks should be valid", func() {
						for _, c := range endpoint.Checks {
							So(c.Type, ShouldBeIn, m.HTTP_CHECK, m.HTTPS_CHECK, m.PING_CHECK)
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
								t.Log(c.Route.Config)
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
	Convey("when requesting to disable all endpoints", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("POST", "/api/v2/endpoints/disable", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)
		addContentTypeHeader(req)

		r.ServeHTTP(resp, req)
		Convey("response should be 200 with DisabledEndpoints map", func() {
			So(resp.Code, ShouldEqual, 200)
			response := rbody.ApiResponse{}
			err := json.Unmarshal(resp.Body.Bytes(), &response)
			So(err, ShouldBeNil)
			So(response.Meta.Code, ShouldEqual, 200)
			So(response.Meta.Type, ShouldEqual, "disabledChecks")
			disabledChecks := make(map[string][]string)
			err = json.Unmarshal(response.Body, &disabledChecks)
			So(err, ShouldBeNil)
			So(disabledChecks, ShouldHaveLength, 4)
			So(disabledChecks["www3_google_com"], ShouldHaveLength, 2)
			So(disabledChecks["www3_google_com"], ShouldContain, "http")
			So(disabledChecks["www3_google_com"], ShouldContain, "ping")
		})
		Convey("response should be 200 with empty disabledEndpoints map", func() {
			So(resp.Code, ShouldEqual, 200)
			response := rbody.ApiResponse{}
			err := json.Unmarshal(resp.Body.Bytes(), &response)
			So(err, ShouldBeNil)
			So(response.Meta.Code, ShouldEqual, 200)
			So(response.Meta.Type, ShouldEqual, "disabledChecks")
			disabledChecks := make(map[string][]string)
			err = json.Unmarshal(response.Body, &disabledChecks)
			So(err, ShouldBeNil)
			So(disabledChecks, ShouldHaveLength, 0)
		})
	})
}
