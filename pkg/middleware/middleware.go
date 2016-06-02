package middleware

import (
	"strconv"
	"strings"

	"github.com/Unknwon/macaron"
	"github.com/raintank/raintank-apps/pkg/auth"
)

type Context struct {
	*macaron.Context
	*auth.SignedInUser
}

func GetContextHandler() macaron.Handler {
	return func(c *macaron.Context) {
		ctx := &Context{
			Context:      c,
			SignedInUser: &auth.SignedInUser{},
		}
		c.Map(ctx)
	}
}

func RequireAdmin() macaron.Handler {
	return func(ctx *Context) {
		if !ctx.IsAdmin {
			ctx.JSON(403, "Permission denied")
		}
	}
}

func RoleAuth(roles ...auth.RoleType) macaron.Handler {
	return func(c *Context) {
		ok := false
		for _, role := range roles {
			if role == c.Role {
				ok = true
				break
			}
		}
		if !ok {
			c.JSON(403, "Permission denied")
		}
	}
}

func Auth(adminKey string) macaron.Handler {
	return func(ctx *Context) {
		key := getApiKey(ctx)
		if key == "" {
			ctx.JSON(401, "Unauthorized")
			return
		}
		user, err := auth.Auth(adminKey, key)
		if err != nil {
			if err == auth.ErrInvalidApiKey {
				ctx.JSON(401, "Unauthorized")
				return
			}
			ctx.JSON(500, err)
			return
		}
		// allow admin users to impersonate other orgs.
		if user.IsAdmin {
			header := ctx.Req.Header.Get("X-Worldping-Org")
			if header != "" {
				orgId, err := strconv.ParseInt(header, 10, 64)
				if err == nil && orgId != 0 {
					user.OrgId = orgId
				}
			}
		}
		ctx.SignedInUser = user
	}
}

func getApiKey(c *Context) string {
	header := c.Req.Header.Get("Authorization")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) == 2 && parts[0] == "Bearer" {
		key := parts[1]
		return key
	}

	return ""
}
