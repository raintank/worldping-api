package api

import (
	"github.com/raintank/worldping-api/pkg/api/rbody"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
)

func GetUsage(c *middleware.Context) *rbody.ApiResponse {
	usage, err := sqlstore.GetUsage()
	if err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("usage", usage)
}

func GetBilling(c *middleware.Context) *rbody.ApiResponse {
	usage := make(map[int64]float64)
	probes, err := sqlstore.GetOnlineProbes()
	if err != nil {
		return rbody.ErrResp(err)
	}
	for _, probe := range probes {
		checks, err := sqlstore.GetProbeChecks(&m.ProbeDTO{Id: probe.Id})
		if err != nil {
			return rbody.ErrResp(err)
		}
		for _, check := range checks {
			if _, ok := usage[check.OrgId]; !ok {
				usage[check.OrgId] = 0
			}
			usage[check.OrgId] += (60.0 / float64(check.Frequency))
		}
	}
	return rbody.OkResp("billing", usage)
}
