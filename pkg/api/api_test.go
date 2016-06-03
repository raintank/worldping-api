package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Unknwon/macaron"
	. "github.com/smartystreets/goconvey/convey"
)

func TestHttpApi(t *testing.T) {

	m := macaron.New()
	Register(m)

	Convey("Given the grafana api", t, func() {
		Convey("request for /foobar", func() {
			resp := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "/foobar", nil)
			So(err, ShouldBeNil)
			m.ServeHTTP(resp, req)
			Convey("should return 404", func() {
				So(resp.Code, ShouldEqual, 404)
			})
		})
		Convey("request for /login", func() {
			resp := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "/login", nil)
			So(err, ShouldBeNil)
			m.ServeHTTP(resp, req)
			Convey("should return 200", func() {
				So(resp.Code, ShouldEqual, 200)
			})
		})
	})
}
