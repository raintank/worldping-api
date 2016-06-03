package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Unknwon/macaron"
	"github.com/go-xorm/xorm"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/services/sqlstore/sqlutil"
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


func TestV1Api(t *testing.T) {
	InitTestDB(t)
	m := macaron.New()
	Register(m)

	Convey("Given raintank collector tags query", t, func() {
		resp, err := executeRaintankDbQuery("raintank_db.tags.collectors.*", 10)
		So(err, ShouldBeNil)

		Convey("should return tags", func() {
			array := resp.([]map[string]interface{})
			So(len(array), ShouldEqual, 2)
			So(array[0]["text"], ShouldEqual, "tag1")
		})
	})