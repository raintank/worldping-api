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
	InitTestDB(t)
	setting.Quota = setting.QuotaSettings{
		Enabled: true,
		Org: &setting.OrgQuota{
			Endpoint: 5,
			Probe:    5,
		},
		Global: &setting.GlobalQuota{
			Endpoint: 5,
			Probe:    5,
		},
	}

	err := AddEndpoint(&m.EndpointDTO{
		Name:  "test1",
		OrgId: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = AddProbe(&m.ProbeDTO{
		Name:  "test1",
		OrgId: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = AddEndpoint(&m.EndpointDTO{
		Name:  "test1",
		OrgId: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = AddProbe(&m.ProbeDTO{
		Name:  "test1",
		OrgId: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	Convey("when org Quota for probes set to 10", t, func() {
		newQuota := m.OrgQuotaDTO{
			OrgId:  1,
			Target: "probe",
			Limit:  10,
		}
		err := UpdateOrgQuota(&newQuota)
		So(err, ShouldBeNil)

		newQuota.OrgId = 4
		err = UpdateOrgQuota(&newQuota)
		So(err, ShouldBeNil)

		Convey("When geting probe quota for org with 1 probe", func() {
			q, err := GetOrgQuotaByTarget(1, "probe", 1)
			So(err, ShouldBeNil)
			So(q.Limit, ShouldEqual, 10)
			So(q.Used, ShouldEqual, 1)
		})
		Convey("When geting probe quota for org with 0 probe", func() {
			q, err := GetOrgQuotaByTarget(4, "probe", 1)
			So(err, ShouldBeNil)
			So(q.Limit, ShouldEqual, 10)
			So(q.Used, ShouldEqual, 0)
		})
		Convey("When getting quota list for org", func() {
			quotas, err := GetOrgQuotas(1)
			So(err, ShouldBeNil)
			So(len(quotas), ShouldEqual, 2)
			for _, res := range quotas {
				limit := 5 //default quota limit
				used := 1
				if res.Target == "probe" {
					limit = 10 //customized quota limit.

				}

				So(res.Limit, ShouldEqual, limit)
				So(res.Used, ShouldEqual, used)

			}
		})
	})
	Convey("when org Quota for probes set to default", t, func() {
		Convey("When geting probe quota for org with 1 probe", func() {
			q, err := GetOrgQuotaByTarget(2, "probe", 3)
			So(err, ShouldBeNil)
			So(q.Limit, ShouldEqual, 3)
			So(q.Used, ShouldEqual, 1)
		})
		Convey("When geting probe quota for org with 0 probe", func() {
			q, err := GetOrgQuotaByTarget(5, "probe", 3)
			So(err, ShouldBeNil)
			So(q.Limit, ShouldEqual, 3)
			So(q.Used, ShouldEqual, 0)
		})
	})

	Convey("When getting global endpoint quota", t, func() {
		q, err := GetGlobalQuotaByTarget("endpoint")
		So(err, ShouldBeNil)

		So(q.Limit, ShouldEqual, 5)
		So(q.Used, ShouldEqual, 2)
	})
	Convey("Should be able to global probe quota", t, func() {
		q, err := GetGlobalQuotaByTarget("probe")
		So(err, ShouldBeNil)

		So(q.Limit, ShouldEqual, 5)
		So(q.Used, ShouldEqual, 2)
	})

}
