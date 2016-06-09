package alerting

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/raintank/raintank-metric/schema"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/metricpublisher"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
)

// Job is a job for an alert execution
// note that LastPointTs is a time denoting the timestamp of the last point to run against
// this way the check runs always on the right data, irrespective of execution delays
// that said, for convenience, we track the generatedAt timestamp
type Job struct {
	OrgId           int64
	CheckId         int64
	EndpointId      int64
	EndpointName    string
	EndpointSlug    string
	Settings        map[string]interface{}
	CheckType       string
	Notifications   m.CheckNotificationSetting
	Freq            int64
	Offset          int64 // offset on top of "even" minute/10s/.. intervals
	Definition      CheckDef
	GeneratedAt     time.Time
	LastPointTs     time.Time
	AssertMinSeries int       // to verify during execution at least this many series are returned (would be nice at some point to include actual number of collectors)
	AssertStart     time.Time // to verify timestamps in response
	AssertStep      int       // to verify step duration
	AssertSteps     int       // to verify during execution this many points are included
}

func (job Job) String() string {
	return fmt.Sprintf("<Job> checkId=%d generatedAt=%s lastPointTs=%s definition: %s", job.CheckId, job.GeneratedAt, job.LastPointTs, job.Definition)
}

func (job Job) StoreResult(res m.CheckEvalResult) {
	if !setting.WriteIndividualAlertResults {
		return
	}
	metrics := make([]*schema.MetricData, 3)
	metricNames := [3]string{"ok_state", "warn_state", "error_state"}
	for pos, state := range metricNames {
		metrics[pos] = &schema.MetricData{
			OrgId:      int(job.OrgId),
			Name:       fmt.Sprintf("health.%s.%s.%s", job.EndpointSlug, strings.ToLower(job.CheckType), state),
			Metric:     fmt.Sprintf("health.%s.%s", strings.ToLower(job.CheckType), state),
			Interval:   int(job.Freq),
			Value:      0.0,
			Unit:       "state",
			Time:       job.LastPointTs.Unix(),
			TargetType: "gauge",
			Tags: []string{
				fmt.Sprintf("endpoint_id:%d", job.EndpointId),
				fmt.Sprintf("monitor_id:%d", job.CheckId),
			},
		}
		metrics[pos].SetId()
	}
	if int(res) >= 0 {
		metrics[int(res)].Value = 1.0
	}
	metricpublisher.Publish(metrics)
}

func (job *Job) assertStart() {
	startTs := job.LastPointTs.Unix() - int64(job.AssertStep*(job.AssertSteps))
	job.AssertStart = time.Unix((startTs+int64(job.AssertStep))-(startTs%int64(job.AssertStep)), 0)
}

// getJobs retrieves all jobs for which lastPointAt % their freq == their offset.
func getJobs(lastPointAt int64) ([]*Job, error) {

	checks, err := sqlstore.GetChecksForAlerts(lastPointAt)
	if err != nil {
		return nil, err
	}

	jobs := make([]*Job, 0)
	for _, monitor := range checks {
		job := buildJobForMonitor(&monitor)
		if job != nil {
			jobs = append(jobs, job)
		}
	}

	return jobs, nil
}

func buildJobForMonitor(check *m.CheckForAlertDTO) *Job {
	//state could in theory be ok, warn, error, but we only use ok vs error for now

	if check.HealthSettings == nil {
		return nil
	}

	if check.Frequency == 0 || check.HealthSettings.Steps == 0 || check.HealthSettings.NumProbes == 0 {
		//fmt.Printf("bad monitor definition given: %#v", monitor)
		return nil
	}

	type Settings struct {
		EndpointSlug string
		CheckType    string
		Duration     string
		NumProbes    int
		Steps        int
	}

	// graphite behaves like so:
	// from is exclusive (from=foo returns data at ts=foo+1 and higher)
	// until is inclusive (until=bar returns data at ts=bar and lower)
	// so if lastPointAt is 1000, and Steps = 3 and Frequency is 10
	// we want points with timestamps 980, 990, 1000
	// we can just query from 970

	settings := Settings{
		EndpointSlug: check.Slug,
		CheckType:    string(check.Type),
		Duration:     fmt.Sprintf("%d", int64(check.HealthSettings.Steps)*check.Frequency),
		NumProbes:    check.HealthSettings.NumProbes,
		Steps:        check.HealthSettings.Steps,
	}

	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	// note: in graphite, using the series-wise sum(), sum(1+null) = 1, and sum(null+null) gets dropped from the result!
	// in bosun, series-wise sum() doesn't exist, you can only sum over time. (though functions like t() help)
	// when bosun pulls in graphite results, null values are removed from the series.
	// we get from graphite the raw (incl nulls) series, so that we can inspect and log/instrument nulls
	// bosun does all the logic as follows: see how many collectors are errorring .Steps in a row, using streak
	// transpose that, to get a 1/0 for each sufficiently-erroring collector, sum them together and compare to the threshold.

	// note: it may look like the end of the queried interval is ambiguous here, and if offset > frequency, may include "too recent" values by accident.
	// fear not, as when we execute the alert in the executor, we set the lastPointTs as end time

	target := `litmus.{{.EndpointSlug}}.*.{{.CheckType | ToLower }}.error_state`
	tpl := `sum(t(streak(graphite("` + target + `", "{{.Duration}}s", "", "")) == {{.Steps}} , "")) >= {{.NumProbes}}`

	var t = template.Must(template.New("query").Funcs(funcMap).Parse(tpl))
	var b bytes.Buffer
	err := t.Execute(&b, settings)
	if err != nil {
		panic(fmt.Sprintf("Could not execute alert query template: %q", err))
	}
	j := &Job{
		CheckId:       check.Id,
		EndpointId:    check.EndpointId,
		EndpointName:  check.Name,
		EndpointSlug:  check.Slug,
		Settings:      check.Settings,
		CheckType:     string(check.Type),
		Notifications: check.HealthSettings.Notifications,
		OrgId:         check.OrgId,
		Freq:          check.Frequency,
		Offset:        check.Offset,
		Definition: CheckDef{
			CritExpr: b.String(),
			WarnExpr: "0", // for now we have only good or bad. so only crit is needed
		},
		AssertMinSeries: check.HealthSettings.NumProbes,
		AssertStep:      int(check.Frequency),
		AssertSteps:     check.HealthSettings.Steps,
	}
	return j
}
