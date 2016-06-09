package api

import (
	"testing"

	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	. "github.com/smartystreets/goconvey/convey"
)

func populateDB(t *testing.T) {
	if err := sqlstore.AddProbe(&m.ProbeDTO{
		Name:  "dev1",
		Tags:  []string{"tag1", "tag2"},
		OrgId: 10,
	}); err != nil {
		t.Fatal(err)
	}

	if err := sqlstore.AddEndpoint(&m.EndpointDTO{
		Name:  "dev2",
		Tags:  []string{"Dev"},
		OrgId: 10,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGraphiteRaintankQueries(t *testing.T) {
	InitTestDB(t)
	populateDB(t)

	Convey("Given raintank collector tags query", t, func() {
		resp, err := executeRaintankDbQuery("raintank_db.tags.collectors.*", 10)
		So(err, ShouldBeNil)

		Convey("should return tags", func() {
			array := resp.([]map[string]interface{})
			So(len(array), ShouldEqual, 2)
			So(array[0]["text"], ShouldEqual, "tag1")
		})
	})

	Convey("Given raintank collector tag values query", t, func() {
		resp, err := executeRaintankDbQuery("raintank_db.tags.collectors.tag1.*", 10)
		So(err, ShouldBeNil)

		Convey("should return tags", func() {
			array := resp.([]map[string]interface{})
			So(len(array), ShouldEqual, 1)
			So(array[0]["text"], ShouldEqual, "dev1")
		})
	})

	Convey("Given raintank endpoint tags query", t, func() {
		resp, err := executeRaintankDbQuery("raintank_db.tags.endpoints.*", 10)
		So(err, ShouldBeNil)

		Convey("should return tags", func() {
			array := resp.([]map[string]interface{})
			So(len(array), ShouldEqual, 1)
			So(array[0]["text"], ShouldEqual, "Dev")
		})
	})

	Convey("Given raintank endpoint tag values query", t, func() {

		resp, err := executeRaintankDbQuery("raintank_db.tags.endpoints.Dev.*", 10)
		So(err, ShouldBeNil)

		Convey("should return tags", func() {
			array := resp.([]map[string]interface{})
			So(len(array), ShouldEqual, 1)
			So(array[0]["text"], ShouldEqual, "dev2")
		})
	})
}
