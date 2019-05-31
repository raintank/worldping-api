package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"

	"github.com/grafana/grafana/pkg/util"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
)

var (
	GraphiteUrl *url.URL
	gProxy      httputil.ReverseProxy
)

func initGraphiteProxy() error {
	GraphiteUrl, err := url.Parse(setting.TsdbUrl + "graphite/")
	if err != nil {
		return err
	}
	gProxy.Director = func(req *http.Request) {
		req.URL.Scheme = GraphiteUrl.Scheme
		req.URL.Host = GraphiteUrl.Host
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/api")
	}
	return nil
}

func V1GraphiteProxy(c *middleware.Context) {
	proxyPath := c.Params("*")

	// check if this is a special raintank_db requests
	if proxyPath == "metrics/find" {
		query := c.Query("query")
		if strings.HasPrefix(query, "raintank_db") {
			response, err := executeRaintankDbQuery(query, int64(c.User.ID))
			if err != nil {
				handleError(c, err)
				return
			}
			c.JSON(200, response)
			return
		}
	}

	// forward to tsdb-gw
	gProxy.ServeHTTP(c.Resp, c.Req.Request)
}

func executeRaintankDbQuery(query string, orgId int64) (interface{}, error) {
	values := []map[string]interface{}{}

	regex := regexp.MustCompile(`^raintank_db\.tags\.(\w+)\.(\w+|\*)`)
	matches := regex.FindAllStringSubmatch(query, -1)

	if len(matches) == 0 {
		return values, nil
	}

	tagType := matches[0][1]
	tagValue := matches[0][2]

	if tagType == "collectors" || tagType == "probes" {
		if tagValue == "*" {
			// return all tags
			tags, err := sqlstore.GetProbeTags(orgId)
			if err != nil {
				return nil, err
			}

			for _, tag := range tags {
				values = append(values, util.DynMap{"text": tag, "expandable": false})
			}
			return values, nil
		} else if tagValue != "" {
			// return tag values for key
			collectorsQuery := m.GetProbesQuery{OrgId: orgId, Tag: tagValue}
			probes, err := sqlstore.GetProbes(&collectorsQuery)
			if err != nil {
				return nil, err
			}
			for _, collector := range probes {
				values = append(values, util.DynMap{"text": collector.Slug, "expandable": false})
			}
		}
	} else if tagType == "endpoints" {
		if tagValue == "*" {
			// return all tags
			tags, err := sqlstore.GetEndpointTags(orgId)
			if err != nil {
				return nil, err
			}

			for _, tag := range tags {
				values = append(values, util.DynMap{"text": tag, "expandable": false})
			}
			return values, nil
		} else if tagValue != "" {
			// return tag values for key
			endpointsQuery := m.GetEndpointsQuery{OrgId: orgId, Tag: tagValue}
			endpoints, err := sqlstore.GetEndpoints(&endpointsQuery)
			if err != nil {
				return nil, err
			}

			for _, endpoint := range endpoints {
				values = append(values, util.DynMap{"text": endpoint.Slug, "expandable": false})
			}

		}
	}

	return values, nil
}
