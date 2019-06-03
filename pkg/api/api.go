package api

import (
	"fmt"

	"github.com/go-macaron/binding"
	"github.com/raintank/tsdb-gw/auth/gcom"
	"github.com/raintank/worldping-api/pkg/api/rbody"
	"github.com/raintank/worldping-api/pkg/elasticsearch"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
	"gopkg.in/macaron.v1"
)

// Register adds http routes
func Register(r *macaron.Macaron) {
	r.Use(macaron.Renderer())
	r.Use(middleware.GetContextHandler())
	reqEditorRole := middleware.RoleAuth(gcom.ROLE_EDITOR, gcom.ROLE_ADMIN)
	quota := middleware.Quota
	bind := binding.Bind
	wrap := rbody.Wrap
	stats := middleware.RequestStats

	// used by LB healthchecks
	r.Get("/login", Heartbeat)

	r.Group("/api/v2", func() {
		r.Get("/quotas", stats("quotas"), wrap(GetQuotas))

		r.Group("/admin", func() {
			r.Group("/quotas", func() {
				r.Get("/:orgId", stats("admin.quotas"), wrap(GetOrgQuotas))
				r.Put("/:orgId/:target/:limit", stats("admin.quotas"), wrap(UpdateOrgQuota))
			})
			r.Get("/usage",stats("admin.usage"), wrap(GetUsage))
			r.Get("/billing", stats("admin.billing"), wrap(GetBilling))
		}, middleware.RequireAdmin())

		r.Group("/endpoints", func() {
			r.Combo("/").
				Get(bind(m.GetEndpointsQuery{}), stats("endpoints"), wrap(GetEndpoints)).
				Post(reqEditorRole, quota("endpoint"), stats("endpoints"), bind(m.EndpointDTO{}), wrap(AddEndpoint)).
				Put(reqEditorRole, stats("endpoints"), bind(m.EndpointDTO{}), wrap(UpdateEndpoint))
			r.Delete("/:id", reqEditorRole, stats("endpoints"), wrap(DeleteEndpoint))
			r.Get("/discover", stats("endpoint_discover"), reqEditorRole, bind(m.DiscoverEndpointCmd{}), wrap(DiscoverEndpoint))
			r.Get("/:id",stats("endpoints"), wrap(GetEndpointById))
			r.Post("/disable", stats("endpoints"), reqEditorRole, wrap(DisableEndpoints))
		})

		r.Group("/probes", func() {
			r.Combo("/").
				Get(bind(m.GetProbesQuery{}), stats("probes"), wrap(GetProbes)).
				Post(reqEditorRole, quota("probe"), stats("probes"), bind(m.ProbeDTO{}), wrap(AddProbe)).
				Put(reqEditorRole, bind(m.ProbeDTO{}), stats("probes"), wrap(UpdateProbe))
			r.Delete("/:id", reqEditorRole, stats("probes"), wrap(DeleteProbe))
			r.Get("/locations", stats("probes"), V1GetCollectorLocations)
			r.Get("/:id", stats("probes"), wrap(GetProbeById))
		})

	}, middleware.Auth(setting.AdminKey))

	r.Get("/_key", middleware.Auth(setting.AdminKey), wrap(GetApiKey))

	// Old v1 api endpoint.
	r.Group("/api", func() {
		// org information available to all users.
		r.Group("/org", func() {
			r.Get("/quotas", stats("v1_quotas"), V1GetOrgQuotas)
		})

		r.Group("/collectors", func() {
			r.Combo("/").
				Get(bind(m.GetProbesQuery{}), stats("v1_probes"), V1GetCollectors).
				Put(reqEditorRole, quota("probe"), stats("v1_probes"), bind(m.ProbeDTO{}), V1AddCollector).
				Post(reqEditorRole, stats("v1_probes"), bind(m.ProbeDTO{}), V1UpdateCollector)
			r.Get("/locations", stats("v1_probes"), V1GetCollectorLocations)
			r.Get("/:id", stats("v1_probes"), V1GetCollectorById)
			r.Delete("/:id", stats("v1_probes"), reqEditorRole, V1DeleteCollector)
		})

		// Monitors
		r.Group("/monitors", func() {
			r.Combo("/").
				Get(bind(m.GetMonitorsQuery{}), stats("v1_monitors"), V1GetMonitors).
				Put(reqEditorRole, stats("v1_monitors"), bind(m.AddMonitorCommand{}), V1AddMonitor).
				Post(reqEditorRole, stats("v1_monitors"), bind(m.UpdateMonitorCommand{}), V1UpdateMonitor)
			r.Delete("/:id", stats("v1_monitors"), reqEditorRole, V1DeleteMonitor)
		})
		// endpoints
		r.Group("/endpoints", func() {
			r.Combo("/").Get(bind(m.GetEndpointsQuery{}), V1GetEndpoints).
				Put(reqEditorRole, quota("endpoint"), stats("v1_endpoints"), bind(m.AddEndpointCommand{}), V1AddEndpoint).
				Post(reqEditorRole, stats("v1_endpoints"), bind(m.UpdateEndpointCommand{}), V1UpdateEndpoint)
			r.Get("/:id", stats("v1_endpoints"), V1GetEndpointById)
			r.Delete("/:id", stats("v1_endpoints"), reqEditorRole, V1DeleteEndpoint)
			r.Get("/discover", stats("v1_endpoint_discover"), reqEditorRole, bind(m.DiscoverEndpointCmd{}), V1DiscoverEndpoint)
		})

		r.Get("/monitor_types", V1GetMonitorTypes)

		//Get Graph data from Graphite.
		r.Any("/graphite/*", stats("graphite"), V1GraphiteProxy)

		//Elasticsearch proxy
		r.Any("/elasticsearch/*", stats("elasticsearch"), V1ElasticsearchProxy)

	}, middleware.Auth(setting.AdminKey))

	// rendering
	//r.Get("/render/*", reqSignedIn, RenderToPng)

	r.Any("/socket.io/", SocketIO)

	r.NotFound(stats("not_found"), NotFoundHandler)

	if err := initGraphiteProxy(); err != nil {
		log.Fatal(4, "API: failed to initialize Graphite Proxy. %s", err)
	}

	if err := elasticsearch.Init(setting.ElasticsearchUrl, "events"); err != nil {
		log.Fatal(4, "API: failed to initialize Elasticsearch Proxy. %s", err)
	}

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
