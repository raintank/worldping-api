package middleware

import (
	"fmt"

	"github.com/Unknwon/macaron"
	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
)

func Quota(target string) macaron.Handler {
	return func(c *Context) {
		limitReached, err := QuotaReached(c, target)
		if err != nil {
			c.JSON(500, fmt.Sprintf("failed to get quota: %s", err))
			return
		}
		if limitReached {
			c.JSON(403, fmt.Sprintf("%s Quota reached", target))
			return
		}
	}
}

func QuotaReached(c *Context, target string) (bool, error) {
	if !setting.Quota.Enabled {
		return false, nil
	}

	// get the list of scopes that this target is valid for. Org, User, Global
	scopes, err := m.GetQuotaScopes(target)
	if err != nil {
		return false, err
	}

	log.Debug("checking quota for %s in scopes %v", target, scopes)

	for _, scope := range scopes {
		log.Debug("checking scope %s", scope.Name)

		switch scope.Name {
		case "global":
			if scope.DefaultLimit < 0 {
				continue
			}
			if scope.DefaultLimit == 0 {
				return true, nil
			}
			quota, err := sqlstore.GetGlobalQuotaByTarget(scope.Target)
			if err != nil {
				return true, err
			}
			if quota.Used >= scope.DefaultLimit {
				return true, nil
			}
		case "org":
			quota, err := sqlstore.GetOrgQuotaByTarget(c.OrgId, scope.Target, scope.DefaultLimit)
			if err != nil {
				return true, err
			}
			if quota.Limit < 0 {
				continue
			}
			if quota.Limit == 0 {
				return true, nil
			}

			if quota.Used >= quota.Limit {
				return true, nil
			}
		}
	}

	return false, nil
}
