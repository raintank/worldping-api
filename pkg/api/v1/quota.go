package v1

import (
	"fmt"

	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
)

func GetOrgQuotas(c *middleware.Context) {
	var quotas []m.OrgQuotaDTO
	var err error
	if setting.Quota.Enabled {
		quotas, err = sqlstore.GetOrgQuotas(c.OrgId)
		if err != nil {
			c.JSON(500, fmt.Sprintf("failed to get quotas. %s", err))
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
