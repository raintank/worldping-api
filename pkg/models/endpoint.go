package models

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

// Typed errors
var (
	ErrEndpointNotFound   = errors.New("Endpoint not found")
	ErrWithMonitorsDelete = errors.New("Endpoint can't be deleted as it still has monitors")
)

type Endpoint struct {
	Id      int64
	OrgId   int64
	Name    string
	Slug    string
	Created time.Time
	Updated time.Time
}

func (endpoint *Endpoint) UpdateSlug() {
	name := strings.ToLower(endpoint.Name)
	re := regexp.MustCompile("[^\\w ]+")
	re2 := regexp.MustCompile("\\s")
	endpoint.Slug = re2.ReplaceAllString(re.ReplaceAllString(name, "_"), "-")
}

type EndpointTag struct {
	Id         int64
	OrgId      int64
	EndpointId int64
	Tag        string
	Created    time.Time
}

type EndpointDTO struct {
	Id      int64     `json:"id"`
	OrgId   int64     `json:"orgId"`
	Name    string    `json:"name" binding:"Required"`
	Slug    string    `json:"slug"`
	Checks  []Check   `json:"checks"`
	Tags    []string  `json:"tags"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

type CheckType string

const (
	HTTP_CHECK  CheckType = "http"
	HTTPS_CHECK CheckType = "https"
	DNS_CHECK   CheckType = "dns"
	PING_CHECK  CheckType = "ping"
)

type Check struct {
	Id             int64                  `json:"id"`
	OrgId          int64                  `json:"orgId"`
	EndpointId     int64                  `json:"endpointId"`
	Route          *CheckRoute            `xorm:"JSON" json:"route"`
	Type           CheckType              `json:"type" binding:"Required,In(http,https,dns,ping)"`
	Frequency      int64                  `json:"frequency" binding:"Required,Range(10,300)"`
	Offset         int64                  `json:"offset"`
	Enabled        bool                   `json:"enabled"`
	State          CheckEvalResult        `json:"state"`
	StateChange    time.Time              `json:"stateChange"`
	StateCheck     time.Time              `json:"stateCheck"`
	Settings       map[string]interface{} `json:"settings" binding:"Required"`
	HealthSettings *CheckHealthSettings   `xorm:"JSON" json:"healthSettings"`
	Created        time.Time              `json:"created"`
	Updated        time.Time              `json:"updated"`
}

type CheckHealthSettings struct {
	NumProbes     int                      `json:"num_collectors" binding:"Required"`
	Steps         int                      `json:"steps" binding:"Required"`
	Notifications CheckNotificationSetting `json:"notifications"`
}

type CheckNotificationSetting struct {
	Enabled   bool   `json:"enabled"`
	Addresses string `json:"addresses"`
}

type RouteType string

const (
	RouteByTags RouteType = "byTags"
	RouteByIds  RouteType = "byIds"
)

type RouteByIdIndex struct {
	Id      int64
	ProbeId int64
	CheckId int64
	Created time.Time
}
type RouteByTagIndex struct {
	Id      int64
	OrgId   int64
	CheckId int64
	Tag     string
	Created time.Time
}

var (
	InvalidRouteConfig = errors.New("Invlid route config")
	UnknownRouteType   = errors.New("unknown route type")
)

type CheckRoute struct {
	Type   RouteType              `json:"type" binding:"Required"`
	Config map[string]interface{} `json:"config"`
}

func (t *CheckRoute) UnmarshalJSON(body []byte) error {
	// use anonamous struct for intermediate decoding.
	// we need to decode the type to know how to decode
	// the config.
	firstPass := struct {
		Type   RouteType       `json:"type"`
		Config json.RawMessage `json:"config"`
	}{}

	err := json.Unmarshal(body, &firstPass)

	if err != nil {
		return err
	}
	config := make(map[string]interface{})

	t.Type = firstPass.Type
	switch firstPass.Type {
	case RouteByTags:
		c := make(map[string][]string)
		err = json.Unmarshal(firstPass.Config, &c)
		if err != nil {
			return err
		}
		for k, v := range c {
			config[k] = v
		}
	case RouteByIds:
		c := make(map[string][]int64)
		err = json.Unmarshal(firstPass.Config, &c)
		if err != nil {
			return err
		}
		for k, v := range c {
			config[k] = v
		}
	default:
		return UnknownRouteType
	}

	t.Config = config
	return err
}

func (r *CheckRoute) Validate() (bool, error) {
	switch r.Type {
	case RouteByTags:
		if len(r.Config) != 1 {
			return false, InvalidRouteConfig
		}
		if _, ok := r.Config["tags"]; !ok {
			return false, InvalidRouteConfig
		}
	case RouteByIds:
		if len(r.Config) != 1 {
			return false, InvalidRouteConfig
		}
		if _, ok := r.Config["ids"]; !ok {
			return false, InvalidRouteConfig
		}
	default:
		return false, UnknownRouteType
	}
	return true, nil
}

// ----------------------
// COMMANDS
type DiscoverEndpointCmd struct {
	Name string `form:"name"`
}

// ---------------------
// QUERIES
type GetEndpointsQuery struct {
	OrgId   int64  `form:"-"`
	Name    string `form:"name"`
	Tag     string `form:"tag"`
	OrderBy string `form:"orderBy" binding:"In(name,slug,created,updated,)"`
}

//Alerting

type CheckState struct {
	Id      int64
	State   CheckEvalResult
	Updated time.Time // this protects against jobs running out of order.
	Checked time.Time
}

type CheckForAlertDTO struct {
	Id             int64
	OrgId          int64
	EndpointId     int64
	Slug           string
	Name           string
	Type           string
	Offset         int64
	Frequency      int64
	Enabled        bool
	StateChange    time.Time
	StateCheck     time.Time
	Settings       map[string]interface{} `xorm:"JSON"`
	HealthSettings *CheckHealthSettings   `xorm:"JSON"`
	Created        time.Time
	Updated        time.Time
}
