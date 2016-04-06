package api

import (
	"github.com/raintank/worldping-api/pkg/bus"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
)

func GetOrgQuotas(c *middleware.Context) Response {
	if !setting.Quota.Enabled {
		return ApiError(404, "Quotas not enabled", nil)
	}
	query := m.GetOrgQuotasQuery{OrgId: c.ParamsInt64(":orgId")}

	if err := bus.Dispatch(&query); err != nil {
		return ApiError(500, "Failed to get org quotas", err)
	}

	return Json(200, query.Result)
}

func GetOwnOrgQuotas(c *middleware.Context) Response {
	if !setting.Quota.Enabled {
		return ApiError(404, "Quotas not enabled", nil)
	}
	query := m.GetOrgQuotasQuery{OrgId: c.OrgId}

	if err := bus.Dispatch(&query); err != nil {
		return ApiError(500, "Failed to get org quotas", err)
	}

	return Json(200, query.Result)
}

func UpdateOrgQuota(c *middleware.Context, cmd m.UpdateOrgQuotaCmd) Response {
	if !setting.Quota.Enabled {
		return ApiError(404, "Quotas not enabled", nil)
	}
	cmd.OrgId = c.ParamsInt64(":orgId")
	cmd.Target = c.Params(":target")

	if _, ok := setting.Quota.Org.ToMap()[cmd.Target]; !ok {
		return ApiError(404, "Invalid quota target", nil)
	}

	if err := bus.Dispatch(&cmd); err != nil {
		return ApiError(500, "Failed to update org quotas", err)
	}
	return ApiSuccess("Organization quota updated")
}
