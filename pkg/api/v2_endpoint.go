package api

import (
	"github.com/raintank/worldping-api/pkg/api/rbody"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/endpointdiscovery"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
)

func GetEndpoints(c *middleware.Context, query m.GetEndpointsQuery) *rbody.ApiResponse {
	query.OrgId = c.OrgId

	endpoints, err := sqlstore.GetEndpoints(&query)
	if err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("endpoints", endpoints)
}

func GetEndpointById(c *middleware.Context) *rbody.ApiResponse {
	id := c.ParamsInt64(":id")

	endpoint, err := sqlstore.GetEndpointById(c.OrgId, id)
	if err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("endpoint", endpoint)
}

func DeleteEndpoint(c *middleware.Context) *rbody.ApiResponse {
	id := c.ParamsInt64(":id")

	err := sqlstore.DeleteEndpoint(c.OrgId, id)
	if err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("endpoint", nil)
}

func AddEndpoint(c *middleware.Context, endpoint m.EndpointDTO) *rbody.ApiResponse {
	endpoint.OrgId = c.OrgId
	if endpoint.Name == "" {
		return rbody.ErrResp(m.NewValidationError("Endpoint name not set."))
	}

	quotas, err := sqlstore.GetOrgQuotas(c.OrgId)
	if err != nil {
		return rbody.ErrResp(m.NewValidationError("Error checking quota"))
	}

	for i := range endpoint.Checks {
		check := endpoint.Checks[i]
		check.OrgId = c.OrgId
		if err := check.Validate(quotas); err != nil {
			return rbody.ErrResp(err)
		}

		err := sqlstore.ValidateCheckRoute(&check)
		if err != nil {
			return rbody.ErrResp(err)
		}
	}

	err := sqlstore.AddEndpoint(&endpoint)
	if err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("endpoint", endpoint)
}

func UpdateEndpoint(c *middleware.Context, endpoint m.EndpointDTO) *rbody.ApiResponse {
	endpoint.OrgId = c.OrgId
	if endpoint.Name == "" {
		return rbody.ErrResp(m.NewValidationError("Endpoint name not set."))
	}
	if endpoint.Id == 0 {
		return rbody.ErrResp(m.NewValidationError("Endpoint id not set."))
	}

	for i := range endpoint.Checks {
		check := endpoint.Checks[i]
		if err := check.Validate(); err != nil {
			return rbody.ErrResp(err)
		}
	}

	err := sqlstore.UpdateEndpoint(&endpoint)
	if err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("endpoint", endpoint)
}

func DiscoverEndpoint(c *middleware.Context, cmd m.DiscoverEndpointCmd) *rbody.ApiResponse {
	endpoint, err := endpointdiscovery.Discover(cmd.Name)
	if err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("endpoint", endpoint)
}

func DisableEndpoints(c *middleware.Context) *rbody.ApiResponse {
	query := m.GetEndpointsQuery{
		OrgId: c.OrgId,
	}

	endpoints, err := sqlstore.GetEndpoints(&query)
	if err != nil {
		return rbody.ErrResp(err)
	}
	disabledChecks := make(map[string][]string)

	for i := range endpoints {
		e := &endpoints[i]
		for j := range e.Checks {
			c := &e.Checks[j]
			if c.Enabled {
				c.Enabled = false
				disabledChecks[e.Slug] = append(disabledChecks[e.Slug], string(c.Type))
			}
		}
		err := sqlstore.UpdateEndpoint(e)
		if err != nil {
			return rbody.ErrResp(err)
		}
	}

	return rbody.OkResp("disabledChecks", disabledChecks)
}
