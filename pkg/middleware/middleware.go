package middleware

import (
	"encoding/json"
	"github.com/Unknwon/macaron"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/log"
	"github.com/raintank/worldping-api/pkg/metrics"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
)

type Context struct {
	*macaron.Context
	*m.SignedInUser

	Session SessionStore

	IsSignedIn     bool
	AllowAnonymous bool
}

func GetContextHandler() macaron.Handler {
	return func(c *macaron.Context) {
		ctx := &Context{
			Context:        c,
			SignedInUser:   &m.SignedInUser{},
			Session:        GetSession(),
			IsSignedIn:     false,
			AllowAnonymous: false,
		}
		// the order in which these are tested are important
		// look for api key in Authorization header first
		// then init session and look for userId in session
		// then look for api key in session (special case for render calls via api)
		// then test if anonymous access is enabled
		/*if initContextWithApiKey(ctx) ||
			initContextWithBasicAuth(ctx) ||
			initContextWithAuthProxy(ctx) ||
			initContextWithUserSessionCookie(ctx) ||
			initContextWithApiKeyFromSession(ctx) ||
			initContextWithAnonymousUser(ctx) {
		}*/
		initContextWithGrafanaNetApiKey(ctx)

		c.Map(ctx)
	}
}

type grafanaNetApiDetails struct {
	OrgSlug   string     `json:"orgSlug"`
	CreatedAt time.Time  `json:"createAt"`
	Name      string     `json:"name"`
	Id        int64      `json:"id"`
	Role      m.RoleType `json:"role"`
	OrgName   string     `json:"orgName"`
	OrgId     int64      `json:"orgId"`
}

func UserFromGrafanaNetApiKey(keyString string) (*m.SignedInUser, error) {
	if keyString == setting.AdminKey {
		return &m.SignedInUser{
			OrgRole:        m.ROLE_ADMIN,
			OrgId:          1,
			OrgName:        "Admin",
			IsGrafanaAdmin: true,
		}, nil
	}

	//validate the API key against grafana.net
	payload := url.Values{}
	payload.Add("token", keyString)
	res, err := http.PostForm("https://grafana.net/api/api-keys/check", payload)

	if err != nil {
		log.Error(3, "failed to check apiKey. %s", err)
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	log.Debug("apiKey check response was: %s", body)
	res.Body.Close()
	if res.StatusCode != 200 {
		return nil, m.ErrInvalidApiKey
	}

	apikey := new(grafanaNetApiDetails)
	err = json.Unmarshal(body, apikey)
	if err != nil {
		log.Error(3, "failed to parse api-keys/check response. %s", err)
		return nil, err
	}

	user := &m.SignedInUser{
		OrgRole: apikey.Role,
		OrgId:   apikey.OrgId,
		OrgName: apikey.OrgName,
	}
	return user, nil
}

func initContextWithGrafanaNetApiKey(ctx *Context) {
	var keyString string
	if keyString = getApiKey(ctx); keyString == "" {
		return
	}
	user, err := UserFromGrafanaNetApiKey(keyString)
	if err != nil {
		return
	}

	ctx.IsSignedIn = true
	ctx.SignedInUser = user
	return
}

// Handle handles and logs error by given status.
func (ctx *Context) Handle(status int, title string, err error) {
	if err != nil {
		log.Error(4, "%s: %v", title, err)
		if setting.Env != setting.PROD {
			ctx.Data["ErrorMsg"] = err
		}
	}

	switch status {
	case 200:
		metrics.M_Page_Status_200.Inc(1)
	case 404:
		metrics.M_Page_Status_404.Inc(1)
	case 500:
		metrics.M_Page_Status_500.Inc(1)
	}

	ctx.Data["Title"] = title
	ctx.HTML(status, strconv.Itoa(status))
}

func (ctx *Context) JsonOK(message string) {
	resp := make(map[string]interface{})

	resp["message"] = message

	ctx.JSON(200, resp)
}

func (ctx *Context) IsApiRequest() bool {
	return strings.HasPrefix(ctx.Req.URL.Path, "/api")
}

func (ctx *Context) JsonApiErr(status int, message string, err error) {
	resp := make(map[string]interface{})

	if err != nil {
		log.Error(4, "%s: %v", message, err)
		if setting.Env != setting.PROD {
			resp["error"] = err.Error()
		}
	}

	switch status {
	case 404:
		metrics.M_Api_Status_404.Inc(1)
		resp["message"] = "Not Found"
	case 500:
		metrics.M_Api_Status_500.Inc(1)
		resp["message"] = "Internal Server Error"
	}

	if message != "" {
		resp["message"] = message
	}

	ctx.JSON(status, resp)
}
