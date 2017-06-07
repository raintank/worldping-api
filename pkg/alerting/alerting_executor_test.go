package alerting

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"bosun.org/graphite"
	"github.com/hashicorp/golang-lru"
	"github.com/raintank/met/helper"
	"github.com/raintank/worldping-api/pkg/alerting/jobqueue"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/raintank/schema.v1"
)

type mockTransport struct {
	queries chan graphite.Request
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	response := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	req.ParseForm()
	start, err := strconv.ParseInt(req.FormValue("from"), 10, 64)
	if err != nil {
		return nil, err
	}
	end, err := strconv.ParseInt(req.FormValue("until"), 10, 64)
	if err != nil {
		return nil, err
	}
	startTs := time.Unix(start, 0)
	endTs := time.Unix(end, 0)
	request := graphite.Request{
		Targets: req.Form["target"],
		Start:   &startTs,
		End:     &endTs,
	}
	m.queries <- request
	response.Header.Set("Content-Type", "application/json")
	responseBody := `
	[
		{"target": "endpoint1", "datapoints": [[0, 1], [0,2], [0,3]]},
		{"target": "endpoint2", "datapoints": [[0, 1], [1,2], [1,3]]}
	]`
	response.Body = ioutil.NopCloser(strings.NewReader(responseBody))
	return response, nil
}

type mockPublisher struct {
}

func (m *mockPublisher) Add(metrics []*schema.MetricData) {
	return
}

func init() {
	metrics, _ := helper.New(false, "", "standard", "worldping-api", "test")
	Init(metrics, &mockPublisher{})
}

func TestExecutor(t *testing.T) {
	transport := &mockTransport{
		queries: make(chan graphite.Request, 10),
	}
	graphite.DefaultClient.Transport = transport
	setting.Alerting.Distributed = false
	ResultQueue = make(chan *m.AlertingJob, 1000)

	Convey("executor must do the right thing", t, func() {
		jobAt := func(ts int64) *m.AlertingJob {
			return &m.AlertingJob{
				CheckForAlertDTO: &m.CheckForAlertDTO{
					HealthSettings: &m.CheckHealthSettings{
						NumProbes: 1,
						Steps:     3,
					},
					Slug:      "test",
					Type:      "http",
					Frequency: 10,
				},
				LastPointTs: time.Unix(ts, 0),
				GeneratedAt: time.Now(),
			}
		}
		jobQ := jobqueue.NewJobQueue()
		cache, err := lru.New(1000)
		if err != nil {
			panic(fmt.Sprintf("Can't create LRU: %s", err.Error()))
		}
		done := make(chan struct{})
		jobsChan := jobQ.Jobs()
		go func() {
			ChanExecutor(jobsChan, cache)
			close(done)
		}()

		jobQ.QueueJob(jobAt(0))
		jobQ.QueueJob(jobAt(1))
		jobQ.QueueJob(jobAt(2))
		jobQ.QueueJob(jobAt(2))
		jobQ.QueueJob(jobAt(1))
		jobQ.QueueJob(jobAt(0))
		jobQ.Close()
		<-done
		close(transport.queries)
		count := int64(0)
		for q := range transport.queries {
			So(q.Targets, ShouldHaveLength, 1)
			So(q.Targets[0], ShouldEqual, "worldping.test.*.http.error_state")
			count++
		}
		So(count, ShouldEqual, 3)
	})
}
