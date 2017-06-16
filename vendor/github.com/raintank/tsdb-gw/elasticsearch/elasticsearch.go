package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/raintank/tsdb-gw/util"
	"github.com/raintank/worldping-api/pkg/log"
	"gopkg.in/macaron.v1"
)

var (
	ElasticsearchUrl *url.URL
	IndexName        string
)

func Init(elasticsearchUrl, indexName string) error {
	var err error
	IndexName = indexName
	ElasticsearchUrl, err = url.Parse(elasticsearchUrl)
	return err
}

func Proxy(orgId int64, c *macaron.Context) {
	proxyPath := c.Params("*")
	body, err := ioutil.ReadAll(c.Req.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, fmt.Sprintf("unable to read request body. %s", err))
		return
	}
	searchBody, err := restrictSearch(orgId, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, fmt.Sprintf("unable to read request body. %s", err))
		return
	}
	log.Debug("search body is: %s", string(searchBody))

	url := new(url.URL)
	*url = *ElasticsearchUrl
	url.Path = util.JoinUrlFragments(ElasticsearchUrl.Path, proxyPath)
	url.RawQuery = c.Req.URL.RawQuery
	request := http.Request{
		Method: "POST",
		URL:    url,
		Body:   ioutil.NopCloser(bytes.NewReader(searchBody)),
	}

	resp, err := http.DefaultClient.Do(&request)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, err.Error())
		return
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, err.Error())
	}
	c.WriteHeader(resp.StatusCode)
	c.Write(respBody)
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
	Query           interface{} `json:"query"`
	Sort            interface{} `json:"sort,omitempty"`
	Fields          interface{} `json:"fields,omitempty"`
	ScriptFields    interface{} `json:"script_fields,omitempty"`
	FielddataFields interface{} `json:"fielddata_fields,omitempty"`
	Aggs            interface{} `json:"aggs,omitempty"`
}

type esQueryWrapper struct {
	Bool esBool `json:"bool"`
}

type esBool struct {
	Must   []interface{} `json:"must"`
	Filter interface{}   `json:"filter"`
}

func transformSearch(orgId int64, search []byte) ([]byte, error) {
	// remove all "format": "epoch_millis" entries, since our timestamp isn't a date
	re := regexp.MustCompile(`,\s*"format"\s*:\s*"epoch_millis"`)
	cleanSearch := re.ReplaceAllLiteral(search, []byte(""))
	re = regexp.MustCompile(`"format"\s*:\s*"epoch_millis"\s*,`)
	cleanSearch = re.ReplaceAllLiteral(cleanSearch, []byte(""))

	s := esSearch{}
	if err := json.Unmarshal(cleanSearch, &s); err != nil {
		return nil, err
	}

	// wrap provided query in a bool query with a filter clause restricting matches to the specified org
	orgCondition := map[string]map[string]int64{"term": {"org_id": orgId}}

	Query := &esQueryWrapper{}
	Query.Bool.Must = append(Query.Bool.Must, s.Query)
	Query.Bool.Filter = orgCondition
	s.Query = Query

	return json.Marshal(s)
}
