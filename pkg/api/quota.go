package api

import (
	"github.com/raintank/worldping-api/pkg/api/rbody"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
)

func V1GetOrgQuotas(c *middleware.Context) {
	var quotas []m.OrgQuotaDTO
	var err error
	if setting.Quota.Enabled {
		quotas, err = sqlstore.GetOrgQuotas(c.OrgId)
		if err != nil {
			handleError(c, err)
			return
		}
	} else {
		quotas = []m.OrgQuotaDTO{
			{
				OrgId:  c.OrgId,
				Target: "endpoint",
				Limit:  -1,
				Used:   -10,
			},
			{
				OrgId:  c.OrgId,
				Target: "probe",
				Limit:  -1,
				Used:   -10,
			},
		}
	}
	c.JSON(200, quotas)
}

func GetQuotas(c *middleware.Context) *rbody.ApiResponse {
	var quotas []m.OrgQuotaDTO
	var err error
	if setting.Quota.Enabled {
		quotas, err = sqlstore.GetOrgQuotas(c.OrgId)
		if err != nil {
			return rbody.ErrResp(err)
		}
	} else {
		quotas = []m.OrgQuotaDTO{
			{
				OrgId:  c.OrgId,
				Target: "endpoint",
				Limit:  -1,
				Used:   -1,
			},
			{
				OrgId:  c.OrgId,
				Target: "probe",
				Limit:  -1,
				Used:   -1,
			},
		}
	}

	return rbody.OkResp("quotas", quotas)
}

func GetOrgQuotas(c *middleware.Context) *rbody.ApiResponse {
	var quotas []m.OrgQuotaDTO
	var err error
	org := c.ParamsInt64("orgId")
	if setting.Quota.Enabled {
		quotas, err = sqlstore.GetOrgQuotas(org)
		if err != nil {
			return rbody.ErrResp(err)
		}
	} else {
		quotas = []m.OrgQuotaDTO{
			{
				OrgId:  org,
				Target: "endpoint",
				Limit:  -1,
				Used:   -1,
			},
			{
				OrgId:  org,
				Target: "probe",
				Limit:  -1,
				Used:   -1,
			},
		}
	}

	return rbody.OkResp("quotas", quotas)
}

func UpdateOrgQuota(c *middleware.Context) *rbody.ApiResponse {
	orgId := c.ParamsInt64(":orgId")
	target := c.Params(":target")
	limit := c.ParamsInt64(":limit")

	if _, ok := setting.Quota.Org.ToMap()[target]; !ok {
		return rbody.ErrResp(m.NewNotFoundError("quota target not found"))
	}

	quota := m.OrgQuotaDTO{
		OrgId:  orgId,
		Target: target,
		Limit:  limit,
	}
	err := sqlstore.UpdateOrgQuota(&quota)
	if err != nil {
		return rbody.ErrResp(err)
	}
	return rbody.OkResp("quota", quota)
}
