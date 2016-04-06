package sqlstore

import (
	"testing"

	"github.com/go-xorm/xorm"
	m "github.com/raintank/worldping-api/pkg/models"
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

	if err := SetEngine(x, false); err != nil {
		t.Fatal(err)
	}
}

func TestQuotaCommandsAndQueries(t *testing.T) {

	Convey("Testing Qutoa commands & queries", t, func() {
		InitTestDB(t)
		orgId := int64(4)

		setting.Quota = setting.QuotaSettings{
			Enabled: true,
			Org: &setting.OrgQuota{
				Endpoint:  5,
				Collector: 5,
			},
			Global: &setting.GlobalQuota{
				Endpoint:  5,
				Collector: 5,
			},
		}

		// create a new org and add user_id 1 as admin.
		// we will then have an org with 1 user. and a user
		// with 1 org.
		collectorCmd := m.AddCollectorCommand{
			OrgId: orgId,
			Name:  "test1",
		}

		err := AddCollector(&collectorCmd)
		So(err, ShouldBeNil)

		Convey("Given saved org quota for users", func() {
			orgCmd := m.UpdateOrgQuotaCmd{
				OrgId:  orgId,
				Target: "collector",
				Limit:  10,
			}
			err := UpdateOrgQuota(&orgCmd)
			So(err, ShouldBeNil)

			Convey("Should be able to get saved quota by org id and target", func() {
				query := m.GetOrgQuotaByTargetQuery{OrgId: orgId, Target: "collector", Default: 1}
				err = GetOrgQuotaByTarget(&query)

				So(err, ShouldBeNil)
				So(query.Result.Limit, ShouldEqual, 10)
			})
			Convey("Should be able to get default quota by org id and target", func() {
				query := m.GetOrgQuotaByTargetQuery{OrgId: 123, Target: "collector", Default: 11}
				err = GetOrgQuotaByTarget(&query)

				So(err, ShouldBeNil)
				So(query.Result.Limit, ShouldEqual, 11)
			})
			Convey("Should be able to get used org quota when rows exist", func() {
				query := m.GetOrgQuotaByTargetQuery{OrgId: orgId, Target: "collector", Default: 11}
				err = GetOrgQuotaByTarget(&query)

				So(err, ShouldBeNil)
				So(query.Result.Used, ShouldEqual, 1)
			})
			Convey("Should be able to get used org quota when no rows exist", func() {
				query := m.GetOrgQuotaByTargetQuery{OrgId: 2, Target: "collector", Default: 11}
				err = GetOrgQuotaByTarget(&query)

				So(err, ShouldBeNil)
				So(query.Result.Used, ShouldEqual, 0)
			})
			Convey("Should be able to quota list for org", func() {
				query := m.GetOrgQuotasQuery{OrgId: orgId}
				err = GetOrgQuotas(&query)

				So(err, ShouldBeNil)
				So(len(query.Result), ShouldEqual, 2)
				for _, res := range query.Result {
					limit := 5 //default quota limit
					used := 0
					if res.Target == "collector" {
						limit = 10 //customized quota limit.
						used = 1
					}
					if res.Target == "endpoint" {
						used = 0
					}

					So(res.Limit, ShouldEqual, limit)
					So(res.Used, ShouldEqual, used)

				}
			})
		})

		Convey("Should be able to global endpoint quota", func() {
			query := m.GetGlobalQuotaByTargetQuery{Target: "endpoint", Default: 5}
			err = GetGlobalQuotaByTarget(&query)
			So(err, ShouldBeNil)

			So(query.Result.Limit, ShouldEqual, 5)
			So(query.Result.Used, ShouldEqual, 0)
		})
		Convey("Should be able to global collector quota", func() {
			query := m.GetGlobalQuotaByTargetQuery{Target: "collector", Default: 5}
			err = GetGlobalQuotaByTarget(&query)
			So(err, ShouldBeNil)

			So(query.Result.Limit, ShouldEqual, 5)
			So(query.Result.Used, ShouldEqual, 1)
		})
	})
}
