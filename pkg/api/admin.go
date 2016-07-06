package api

import (
	"github.com/raintank/worldping-api/pkg/api/rbody"
	"github.com/raintank/worldping-api/pkg/middleware"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
)

func GetUsage(c *middleware.Context) *rbody.ApiResponse {
	usage, err := sqlstore.GetUsage()
	if err != nil {
		return rbody.ErrResp(err)
	}

	return rbody.OkResp("usage", usage)
}
