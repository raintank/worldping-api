package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/util"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/middleware"
	"github.com/raintank/worldping-api/pkg/setting"
)

func V1ElasticsearchProxy(c *middleware.Context) {
	proxyPath := c.Params("*")
	target, _ := url.Parse(setting.ElasticsearchUrl)

	y, m, d := time.Now().Date()
	idxDate := fmt.Sprintf("events-%d-%02d-%02d", y, m, d)

	// check if this is a special raintank_db requests
	if c.Req.Request.Method == "GET" && proxyPath == fmt.Sprintf("%s/_stats", idxDate) {
		c.JSON(200, "ok")
		return
	}

	if c.Req.Request.Method == "POST" && proxyPath == "_msearch" {
		body, err := ioutil.ReadAll(c.Req.Request.Body)
		if err != nil {
			c.JSON(400, fmt.Sprintf("unable to read request body. %s", err))
			return
		}
		searchBody, err := restrictSearch(c.OrgId, body)
		if err != nil {
			c.JSON(400, fmt.Sprintf("unable to handle request body. %s", err))
			return
		}
		log.Debug("search body is: %s", string(searchBody))

		director := func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = util.JoinUrlFragments(target.Path, proxyPath)

			req.Body = ioutil.NopCloser(bytes.NewReader(searchBody))
			req.ContentLength = int64(len(searchBody))
			req.Header.Set("Content-Length", strconv.FormatInt(req.ContentLength, 10))
		}

		proxy := &httputil.ReverseProxy{Director: director}

		proxy.ServeHTTP(c.RW(), c.Req.Request)
		return
	}

	c.JSON(404, "Not Found")
}

func restrictSearch(orgId int64, body []byte) ([]byte, error) {
	var newBody bytes.Buffer

	lines := strings.Split(string(body), "\n")
	for i := 0; i < len(lines); i += 2 {
		if lines[i] == "" {
			continue
		}
		if err := validateHeader([]byte(lines[i])); err != nil {
			return newBody.Bytes(), err
		}
		newBody.Write([]byte(lines[i] + "\n"))

		s, err := transformSearch(orgId, []byte(lines[i+1]))
		if err != nil {
			return newBody.Bytes(), err
		}
		newBody.Write(s)
		newBody.Write([]byte("\n"))
	}
	return newBody.Bytes(), nil
}

type msearchHeader struct {
	SearchType        string   `json:"search_type"`
	IgnoreUnavailable bool     `json:"ignore_unavailable,omitempty"`
	Index             []string `json:"index"`
}

func validateHeader(header []byte) error {
	h := msearchHeader{}
	log.Debug("validating search header: %s", string(header))
	if err := json.Unmarshal(header, &h); err != nil {
		return err
	}
	if h.SearchType != "query_then_fetch" && h.SearchType != "count" {
		return fmt.Errorf("invalid search_type %s", h.SearchType)
	}

	for _, index := range h.Index {
		if match, err := regexp.Match("^events-\\d\\d\\d\\d-\\d\\d-\\d\\d$", []byte(index)); err != nil || !match {
			return fmt.Errorf("invalid index name. %s", index)
		}
	}

	return nil
}

type esSearch struct {
	Size            int         `json:"size"`
	Query           esQuery     `json:"query"`
	Sort            interface{} `json:"sort,omitempty"`
	Fields          interface{} `json:"fields,omitempty"`
	ScriptFields    interface{} `json:"script_fields,omitempty"`
	FielddataFields interface{} `json:"fielddata_fields,omitempty"`
	Aggs            interface{} `json:"aggs,omitempty"`
}

type esQuery struct {
	Filtered esFiltered `json:"filtered"`
}

type esFiltered struct {
	Query  interface{} `json:"query"`
	Filter esFilter    `json:"filter"`
}

type esFilter struct {
	Bool esBool `json:"bool"`
}

type esBool struct {
	Must []interface{} `json:"must"`
}

func transformSearch(orgId int64, search []byte) ([]byte, error) {
	s := esSearch{}
	if err := json.Unmarshal(search, &s); err != nil {
		return nil, err
	}

	orgCondition := map[string]map[string]int64{"term": {"org_id": orgId}}

	s.Query.Filtered.Filter.Bool.Must = append(s.Query.Filtered.Filter.Bool.Must, orgCondition)

	return json.Marshal(s)
}
