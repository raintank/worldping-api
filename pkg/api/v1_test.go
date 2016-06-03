package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Unknwon/macaron"
	"github.com/go-xorm/xorm"
	"github.com/raintank/worldping-api/pkg/api/rbody"
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

	if err != nil {
		t.Fatalf("Failed to init in memory sqllite3 db %v", err)
	}

	sqlutil.CleanDB(x)

	if err := sqlstore.SetEngine(x, true); err != nil {
		t.Fatal(err)
	}
}

func addAuthHeader(req *http.Request) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", setting.AdminKey))
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

					for i, _ := range []int{1, 2, 3} {
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
					for i, _ := range []int{1, 2, 3} {
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

					for i, _ := range []int{1, 2, 3} {
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
					for i, _ := range []int{1, 2, 3} {
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
