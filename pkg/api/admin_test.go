package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Unknwon/macaron"
	"github.com/raintank/worldping-api/pkg/api/rbody"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
	. "github.com/smartystreets/goconvey/convey"
)

func TestUsageApi(t *testing.T) {
	InitTestDB(t)
	r := macaron.Classic()
	setting.AdminKey = "test"
	Register(r)
	populateEndpoints(t)
	populateCollectors(t)

	Convey("When getting usage stats", t, func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/v2/admin/usage", nil)
		So(err, ShouldBeNil)
		addAuthHeader(req)

		r.ServeHTTP(resp, req)
		Convey("should return 200", func() {
			So(resp.Code, ShouldEqual, 200)
			Convey("usage response should be valid", func() {
				response := rbody.ApiResponse{}
				err := json.Unmarshal(resp.Body.Bytes(), &response)
				So(err, ShouldBeNil)

				So(response.Meta.Code, ShouldEqual, 200)
				So(response.Meta.Type, ShouldEqual, "usage")
				So(response.Meta.Message, ShouldEqual, "success")
				Convey("response body should be usage data", func() {
					usage := m.Usage{}
					err := json.Unmarshal(response.Body, &usage)
					So(err, ShouldBeNil)
					So(usage.Endpoints.Total, ShouldEqual, 6)
				})
			})
		})
	})

}
