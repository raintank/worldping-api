package api

import (
	"github.com/raintank/worldping-api/pkg/api/rbody"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
)

func GetProbes(c *middleware.Context, query m.GetProbesQuery) *rbody.ApiResponse {
	query.OrgId = c.OrgId

	probes, err := sqlstore.GetProbes(&query)
	if err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("probes", probes)
}

func GetProbeById(c *middleware.Context) *rbody.ApiResponse {
	id := c.ParamsInt64(":id")

	probe, err := sqlstore.GetProbeById(id, c.OrgId)
	if err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("probe", probe)
}

func DeleteProbe(c *middleware.Context) *rbody.ApiResponse {
	id := c.ParamsInt64(":id")

	err := sqlstore.DeleteProbe(id, c.OrgId)
	if err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("probe", nil)
}

func AddProbe(c *middleware.Context, probe m.ProbeDTO) *rbody.ApiResponse {
	probe.OrgId = c.OrgId
	if probe.Id != 0 {
		return rbody.ErrResp(m.NewValidationError("Id already set. Try update instead of create."))
	}
	if probe.Name == "" {
		return rbody.ErrResp(m.NewValidationError("Probe name not set."))
	}
	if probe.Public {
		if !c.IsAdmin {
			return rbody.ErrResp(m.NewValidationError("Only admins can make public probes."))
		}
	}

	if err := sqlstore.AddProbe(&probe); err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("probe", probe)
}

func UpdateProbe(c *middleware.Context, probe m.ProbeDTO) *rbody.ApiResponse {
	probe.OrgId = c.OrgId
	if probe.Name == "" {
		return rbody.ErrResp(m.NewValidationError("Probe name not set."))
	}

	if err := sqlstore.UpdateProbe(&probe); err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("probe", probe)
}
