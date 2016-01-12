package elasticstore

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/bus"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/setting"
	elastigo "github.com/mattbaird/elastigo/lib"
	"github.com/raintank/raintank-metric/schema"
)

func init() {
	bus.AddHandler("es", GetEventsQuery)
}

func GetEventsQuery(query *m.GetEventsQuery) error {
	query.Result = make([]*schema.ProbeEvent, 0)
	esQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"filtered": map[string]interface{}{
				"filter": map[string]interface{}{
					"and": []map[string]interface{}{
						{
							"range": map[string]interface{}{
								"timestamp": map[string]interface{}{
									"gte": query.Start,
									"lte": query.End,
								},
							},
						},
						{
							"term": map[string]int64{
								"org_id": query.OrgId,
							},
						},
					},
				},
				"query": map[string]interface{}{
					"query_string": map[string]interface{}{
						"query":                    query.Query,
						"lowercase_expanded_terms": false,
					},
				},
			},
		},
	}
	var out elastigo.SearchResult
	var err error

	wildcard := setting.Cfg.Section("elasticsearch").Key("wildcard_events").MustBool()

	if wildcard {
		out, err = es.Search("events*", "", map[string]interface{}{"size": query.Size, "sort": "timestamp:desc"}, esQuery)
	} else {
		//TODO(awoods): this needs optimizations for when the requested time range is very large. The index names are
		// used in the url sent to Elasticsearch, and the url is limited to 4096bytes.  At 18bytes per index name, that is only
		// 227days of data.  So if you try to request more the 227days of data, ES will return an Internal Server Error.
		current := time.Unix(query.Start/1000, 0)
		end := time.Unix(query.End/1000, 0)
		idxDates := make([]string, 0)
		for current.Unix() <= end.Unix() {
			y, m, d := current.Date()
			idxDates = append(idxDates, fmt.Sprintf("events-%d-%02d-%02d", y, m, d))
			current = current.Add(time.Hour * 24)
		}
		if len(idxDates) < 1 {
			return fmt.Errorf("Failed to get index list to query.")
		}
		allTogether := strings.Join(idxDates, ",")

		out, err = es.Search(allTogether, "", map[string]interface{}{"size": query.Size, "sort": "timestamp:desc", "ignore_unavailable": true}, esQuery)
	}

	if err != nil {
		return err
	}
	for _, hit := range out.Hits.Hits {
		var source schema.ProbeEvent
		err = json.Unmarshal(*hit.Source, &source)
		if err != nil {
			return err
		}
		query.Result = append(query.Result, &source)
	}

	return nil
}
