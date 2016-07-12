package api

import (
	"fmt"

	"github.com/Unknwon/macaron"
	"github.com/macaron-contrib/binding"
	"github.com/raintank/raintank-apps/pkg/auth"
	"github.com/raintank/worldping-api/pkg/api/rbody"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
)

// Register adds http routes
func Register(r *macaron.Macaron) {
	r.Use(macaron.Renderer())
	r.Use(middleware.GetContextHandler())
	reqEditorRole := middleware.RoleAuth(auth.ROLE_EDITOR, auth.ROLE_ADMIN)
	quota := middleware.Quota
	bind := binding.Bind
	wrap := rbody.Wrap

	// used by LB healthchecks
	r.Get("/login", Heartbeat)

	r.Group("/api/v2", func() {
		r.Get("/quotas", wrap(GetQuotas))

		r.Group("/admin", func() {
			r.Group("/quotas", func() {
				r.Get("/:orgId", wrap(GetOrgQuotas))
				r.Put("/:orgId/:target/:limit", wrap(UpdateOrgQuota))
			})
			r.Get("/usage", wrap(GetUsage))
			r.Get("/billing", wrap(GetBilling))
		}, middleware.RequireAdmin())

		r.Group("/endpoints", func() {
			r.Combo("/").
				Get(bind(m.GetEndpointsQuery{}), wrap(GetEndpoints)).
				Post(reqEditorRole, quota("endpoint"), bind(m.EndpointDTO{}), wrap(AddEndpoint)).
				Put(reqEditorRole, bind(m.EndpointDTO{}), wrap(UpdateEndpoint))
			r.Delete("/:id", reqEditorRole, wrap(DeleteEndpoint))
			r.Get("/discover", reqEditorRole, bind(m.DiscoverEndpointCmd{}), wrap(DiscoverEndpoint))
			r.Get("/:id", wrap(GetEndpointById))
		})

		r.Group("/probes", func() {
			r.Combo("/").
				Get(bind(m.GetProbesQuery{}), wrap(GetProbes)).
				Post(reqEditorRole, quota("probe"), bind(m.ProbeDTO{}), wrap(AddProbe)).
				Put(reqEditorRole, bind(m.ProbeDTO{}), wrap(UpdateProbe))
			r.Delete("/:id", reqEditorRole, wrap(DeleteProbe))
			r.Get("/locations", V1GetCollectorLocations)
			r.Get("/:id", wrap(GetProbeById))
		})

	}, middleware.Auth(setting.AdminKey))

	// Old v1 api endpoint.
	r.Group("/api", func() {
		// org information available to all users.
		r.Group("/org", func() {
			r.Get("/quotas", V1GetOrgQuotas)
		})

		r.Group("/collectors", func() {
			r.Combo("/").
				Get(bind(m.GetProbesQuery{}), V1GetCollectors).
				Put(reqEditorRole, quota("probe"), bind(m.ProbeDTO{}), V1AddCollector).
				Post(reqEditorRole, bind(m.ProbeDTO{}), V1UpdateCollector)
			r.Get("/locations", V1GetCollectorLocations)
			r.Get("/:id", V1GetCollectorById)
			r.Delete("/:id", reqEditorRole, V1DeleteCollector)
		})

		// Monitors
		r.Group("/monitors", func() {
			r.Combo("/").
				Get(bind(m.GetMonitorsQuery{}), V1GetMonitors).
				Put(reqEditorRole, bind(m.AddMonitorCommand{}), V1AddMonitor).
				Post(reqEditorRole, bind(m.UpdateMonitorCommand{}), V1UpdateMonitor)
			r.Delete("/:id", reqEditorRole, V1DeleteMonitor)
		})
		// endpoints
		r.Group("/endpoints", func() {
			r.Combo("/").Get(bind(m.GetEndpointsQuery{}), V1GetEndpoints).
				Put(reqEditorRole, quota("endpoint"), bind(m.AddEndpointCommand{}), V1AddEndpoint).
				Post(reqEditorRole, bind(m.UpdateEndpointCommand{}), V1UpdateEndpoint)
			r.Get("/:id", V1GetEndpointById)
			r.Delete("/:id", reqEditorRole, V1DeleteEndpoint)
			r.Get("/discover", reqEditorRole, bind(m.DiscoverEndpointCmd{}), V1DiscoverEndpoint)
		})

		r.Get("/monitor_types", V1GetMonitorTypes)

		//Get Graph data from Graphite.
		r.Any("/graphite/*", V1GraphiteProxy)

		//Elasticsearch proxy
		r.Any("/elasticsearch/*", V1ElasticsearchProxy)

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
	err := sqlstore.TestDB()
	if err != nil {
		c.JSON(500, err.Error)
	}
	c.JSON(200, "OK")
	return
}

func handleError(c *middleware.Context, err error) {
	if e, ok := err.(m.AppError); ok {
		c.JSON(e.Code(), e.Message())
		return
	}
	log.Error(3, "%s. %s", c.Req.RequestURI, err)
	c.JSON(500, fmt.Sprintf("Fatal error. %s", err))
}
