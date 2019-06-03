package middleware

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafana/metrictank/stats"
	"github.com/raintank/tsdb-gw/auth"
	"github.com/raintank/tsdb-gw/auth/gcom"
	"gopkg.in/macaron.v1"
)

type Context struct {
	*macaron.Context
	*auth.User
	ApiKey string
}

type requestStats struct {
	sync.Mutex
	responseCounts    map[string]map[int]*stats.CounterRate32
	latencyHistograms map[string]*stats.LatencyHistogram15s32
	sizeMeters        map[string]*stats.Meter32
}

func (r *requestStats) StatusCount(handler string, status int) {
	metricKey := fmt.Sprintf("api.request.%s.status.%d", handler, status)
	r.Lock()
	p, ok := r.responseCounts[handler]
	if !ok {
		p = make(map[int]*stats.CounterRate32)
		r.responseCounts[handler] = p
	}
	c, ok := p[status]
	if !ok {
		c = stats.NewCounterRate32(metricKey)
		p[status] = c
	}
	r.Unlock()
	c.Inc()
}

func (r *requestStats) Latency(handler string, dur time.Duration) {
	r.Lock()
	p, ok := r.latencyHistograms[handler]
	if !ok {
		p = stats.NewLatencyHistogram15s32(fmt.Sprintf("api.request.%s", handler))
		r.latencyHistograms[handler] = p
	}
	r.Unlock()
	p.Value(dur)
}

func (r *requestStats) PathSize(handler string, size int) {
	r.Lock()
	p, ok := r.sizeMeters[handler]
	if !ok {
		p = stats.NewMeter32(fmt.Sprintf("api.request.%s.size", handler), false)
		r.sizeMeters[handler] = p
	}
	r.Unlock()
	p.Value(size)
}

// RequestStats returns a middleware that tracks request metrics.
func RequestStats(handler string) macaron.Handler {
	return func(ctx *Context) {
		if handlerStats == nil {
			return
		}
		start := time.Now()
		rw := ctx.Resp.(macaron.ResponseWriter)
		// call next handler. This will return after all handlers
		// have completed and the request has been sent.
		ctx.Next()
		status := rw.Status()
		handlerStats.StatusCount(handler, status)
		handlerStats.Latency(handler, time.Since(start))
		// only record the request size if the request succeeded.
		if status < 300 {
			handlerStats.PathSize(handler, rw.Size())
		}
	}
}

var authPlugin auth.AuthPlugin
var handlerStats *requestStats

func Init(adminKey string) {
	authPlugin = auth.GetAuthPlugin("grafana")
	auth.AdminKey = adminKey
	handlerStats = &requestStats{
		responseCounts:    make(map[string]map[int]*stats.CounterRate32),
		latencyHistograms: make(map[string]*stats.LatencyHistogram15s32),
		sizeMeters:        make(map[string]*stats.Meter32),
	}
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
	return authPlugin.Auth("api_key", key)
}

func Auth(adminKey string) macaron.Handler {
	if authPlugin == nil {
		Init(adminKey)
	}
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
