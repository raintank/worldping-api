package elasticstore

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/bus"
	m "github.com/grafana/grafana/pkg/models"
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
					"query_string": map[string]string{
						"query": query.Query,
					},
				},
			},
		},
	}
	start := time.Unix(query.Start / 1000, 0)
	end := time.Unix(query.End / 1000, 0)
	r := end.Sub(start) / time.Hour / 24
	idxDates := make([]string, 0, r + 1)
	y, m, d := start.Date()
	if r > 0 {
		for {
			end = end.Add(-(time.Hour * 24))
			y2, m2, d2 := end.Date()
			idxDates = append(idxDates, fmt.Sprintf("events-%d-%d-%d", y, m, d))
			if y2 <= y && m2 <= m && d2 <= d {
				break
			}
		}
	}
	allTogether := strings.Join(idxDates, ",")
	out, err := es.Search(allTogether, "", map[string]interface{}{"size": query.Size, "sort": "timestamp:desc"}, esQuery)
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
