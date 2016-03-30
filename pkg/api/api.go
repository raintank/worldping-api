package api

import (
	"github.com/Unknwon/macaron"
	"github.com/grafana/grafana/pkg/middleware"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/macaron-contrib/binding"
)

// Register adds http routes
func Register(r *macaron.Macaron) {
	reqSignedIn := middleware.Auth(&middleware.AuthOptions{ReqSignedIn: true})
	reqGrafanaAdmin := middleware.Auth(&middleware.AuthOptions{ReqSignedIn: true, ReqGrafanaAdmin: true})
	reqEditorRole := middleware.RoleAuth(m.ROLE_EDITOR, m.ROLE_ADMIN)
	quota := middleware.Quota
	bind := binding.Bind

	// authed api
	r.Group("/api", func() {
		// org information available to all users.
		r.Group("/org", func() {
			r.Get("/quotas", wrap(GetOwnOrgQuotas))
		})

		// orgs (admin routes)
		r.Group("/orgs/:orgId", func() {
			r.Get("/quotas", wrap(GetOrgQuotas))
			r.Put("/quotas/:target", bind(m.UpdateOrgQuotaCmd{}), wrap(UpdateOrgQuota))
		}, reqGrafanaAdmin)

		r.Group("/collectors", func() {
			r.Combo("/").
				Get(bind(m.GetCollectorsQuery{}), wrap(GetCollectors)).
				Put(reqEditorRole, quota("collector"), bind(m.AddCollectorCommand{}), wrap(AddCollector)).
				Post(reqEditorRole, bind(m.UpdateCollectorCommand{}), wrap(UpdateCollector))
			r.Get("/:id", wrap(GetCollectorById))
			r.Delete("/:id", reqEditorRole, wrap(DeleteCollector))
		})

		// Monitors
		r.Group("/monitors", func() {
			r.Combo("/").
				Get(bind(m.GetMonitorsQuery{}), wrap(GetMonitors)).
				Put(reqEditorRole, bind(m.AddMonitorCommand{}), wrap(AddMonitor)).
				Post(reqEditorRole, bind(m.UpdateMonitorCommand{}), wrap(UpdateMonitor))
			r.Get("/:id", wrap(GetMonitorById))
			r.Delete("/:id", reqEditorRole, wrap(DeleteMonitor))
		})
		// endpoints
		r.Group("/endpoints", func() {
			r.Combo("/").Get(bind(m.GetEndpointsQuery{}), wrap(GetEndpoints)).
				Put(reqEditorRole, quota("endpoint"), bind(m.AddEndpointCommand{}), wrap(AddEndpoint)).
				Post(reqEditorRole, bind(m.UpdateEndpointCommand{}), wrap(UpdateEndpoint))
			r.Get("/:id", wrap(GetEndpointById))
			r.Delete("/:id", reqEditorRole, wrap(DeleteEndpoint))
			r.Get("/discover", reqEditorRole, bind(m.EndpointDiscoveryCommand{}), wrap(DiscoverEndpoint))
		})

		r.Get("/monitor_types", wrap(GetMonitorTypes))

		//Events
		r.Get("/events", bind(m.GetEventsQuery{}), wrap(GetEvents))

		//Get Graph data from Graphite.
		r.Any("/graphite/*", GraphiteProxy)

		//Elasticsearch proxy
		r.Any("/elasticsearch/*", ElasticsearchProxy)

	}, reqSignedIn)

	// rendering
	//r.Get("/render/*", reqSignedIn, RenderToPng)

	r.Any("/socket.io/", SocketIO)

	r.NotFound(NotFoundHandler)
}

func NotFoundHandler(c *middleware.Context) {
	c.JsonApiErr(404, "Not found", nil)
	return
}
