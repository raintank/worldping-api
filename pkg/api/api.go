package api

import (
	"github.com/Unknwon/macaron"
	"github.com/macaron-contrib/binding"
	"github.com/raintank/raintank-apps/pkg/auth"
	v1 "github.com/raintank/worldping-api/pkg/api/v1"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
)

// Register adds http routes
func Register(r *macaron.Macaron) {
	r.Use(middleware.GetContextHandler())
	reqEditorRole := middleware.RoleAuth(auth.ROLE_EDITOR, auth.ROLE_ADMIN)
	quota := middleware.Quota
	bind := binding.Bind

	// used by LB healthchecks
	r.Get("/login", Heartbeat)

	r.Group("/api/v2", func() {
		r.Get("/quotas", GetQuotas)
		r.Group("/admin", func() {
			r.Group("/quotas", func() {
				r.Get("/:orgId", GetOrgQuotas)
				r.Put("/:orgId/:target/:limit", UpdateOrgQuota)
			})
		}, middleware.RequireAdmin())

	}, middleware.Auth(setting.AdminKey))

	// Old v1 api endpoint.
	r.Group("/api", func() {
		// org information available to all users.
		r.Group("/org", func() {
			r.Get("/quotas", v1.GetOrgQuotas)
		})

		r.Group("/collectors", func() {
			r.Combo("/").
				Get(bind(m.GetProbesQuery{}), v1.GetCollectors).
				Put(reqEditorRole, quota("probe"), bind(m.ProbeDTO{}), v1.AddCollector).
				Post(reqEditorRole, bind(m.ProbeDTO{}), v1.UpdateCollector)
			r.Get("/locations", v1.GetCollectorLocations)
			r.Get("/:id", v1.GetCollectorById)
			r.Delete("/:id", reqEditorRole, v1.DeleteCollector)
		})

		// Monitors
		r.Group("/monitors", func() {
			r.Combo("/").
				Get(bind(m.GetMonitorsQuery{}), v1.GetMonitors).
				Put(reqEditorRole, bind(m.AddMonitorCommand{}), v1.AddMonitor).
				Post(reqEditorRole, bind(m.UpdateMonitorCommand{}), v1.UpdateMonitor)
			r.Delete("/:id", reqEditorRole, v1.DeleteMonitor)
		})
		// endpoints
		r.Group("/endpoints", func() {
			r.Combo("/").Get(bind(m.GetEndpointsQuery{}), v1.GetEndpoints).
				Put(reqEditorRole, quota("endpoint"), bind(m.AddEndpointCommand{}), v1.AddEndpoint).
				Post(reqEditorRole, bind(m.UpdateEndpointCommand{}), v1.UpdateEndpoint)
			r.Get("/:id", v1.GetEndpointById)
			r.Delete("/:id", reqEditorRole, v1.DeleteEndpoint)
			r.Get("/discover", reqEditorRole, bind(m.DiscoverEndpointCmd{}), v1.DiscoverEndpoint)
		})

		r.Get("/monitor_types", v1.GetMonitorTypes)

		//Get Graph data from Graphite.
		r.Any("/graphite/*", v1.GraphiteProxy)

		//Elasticsearch proxy
		r.Any("/elasticsearch/*", v1.ElasticsearchProxy)

	}, middleware.Auth(setting.AdminKey))

	// rendering
	//r.Get("/render/*", reqSignedIn, RenderToPng)

	r.Any("/socket.io/", SocketIO)

	r.NotFound(NotFoundHandler)
}

func NotFoundHandler(c *middleware.Context) {
	c.JSON(404, "Not found")
	return
}

func Heartbeat(c *middleware.Context) {
	c.JSON(200, "OK")
	return
}
