package middleware

import (
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/raintank/tsdb-gw/auth"
	"github.com/raintank/tsdb-gw/auth/gcom"
	"gopkg.in/macaron.v1"
)

type Context struct {
	*macaron.Context
	*auth.User
	ApiKey string
}

var authPlugin auth.AuthPlugin

func init() {
	authPlugin = auth.NewGrafanaComAuth()
}

func GetContextHandler() macaron.Handler {
	return func(c *macaron.Context) {
		ctx := &Context{
			Context: c,
			User:    &auth.User{},
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

func RoleAuth(roles ...gcom.RoleType) macaron.Handler {
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

func GetUser(adminKey, key string) (*auth.User, error) {
	return authPlugin.Auth(adminKey, key)
}

func Auth(adminKey string) macaron.Handler {
	return func(ctx *Context) {
		key, err := getApiKey(ctx)
		if err != nil {
			ctx.JSON(401, "Invalid Authentication header.")
			return
		}
		if key == "" {
			ctx.JSON(401, "Unauthorized")
			return
		}

		user, err := GetUser(adminKey, key)
		if err != nil {
			if err == auth.ErrInvalidCredentials {
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
					user.ID = int(orgId)
				}
			}
		}
		ctx.User = user
		ctx.ApiKey = key
	}
}

func getApiKey(c *Context) (string, error) {
	header := c.Req.Header.Get("Authorization")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) == 2 && parts[0] == "Bearer" {
		key := parts[1]
		return key, nil
	}

	if len(parts) == 2 && parts[0] == "Basic" {
		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return "", err
		}
		userAndPass := strings.SplitN(string(decoded), ":", 2)
		if userAndPass[0] == "api_key" {
			return userAndPass[1], nil
		}
	}

	return "", nil
}
