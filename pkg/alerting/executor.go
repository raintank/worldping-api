package alerting

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/log"
	"github.com/hashicorp/golang-lru"
	"github.com/raintank/worldping-api/pkg/graphite"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/notifications"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"

	bgraphite "bosun.org/graphite"
)

type GraphiteReturner func(org_id int64) (bgraphite.Context, error)

func GraphiteAuthContextReturner(org_id int64) (bgraphite.Context, error) {
	u, err := url.Parse(setting.GraphiteUrl)
	if err != nil {
		return nil, fmt.Errorf("could not parse graphiteUrl: %q", err)
	}
	u.Path = path.Join(u.Path, "render/")
	ctx := graphite.GraphiteContext{
		Host: u.String(),
		Header: http.Header{
			"X-Org-Id":   []string{fmt.Sprintf("%d", org_id)},
			"User-Agent": []string{"grafana alert-executor"},
		},
		Traces: make([]graphite.Trace, 0),
	}
	return &ctx, nil
}

func ChanExecutor(fn GraphiteReturner, jobQueue <-chan *Job, cache *lru.Cache) {
	executorNum.Inc(1)
	defer executorNum.Dec(1)

	for j := range jobQueue {
		go func(job *Job) {
			jobQueueInternalItems.Value(int64(len(jobQueue)))
			jobQueueInternalSize.Value(int64(setting.InternalJobQueueSize))
			if setting.AlertingInspect {
				inspect(fn, job, cache)
			} else {
				execute(fn, job, cache)
			}
		}(j)
	}
}

func inspect(fn GraphiteReturner, job *Job, cache *lru.Cache) {
	key := fmt.Sprintf("%d-%d", job.CheckId, job.LastPointTs.Unix())
	if found, _ := cache.ContainsOrAdd(key, true); found {
		//log.Debug("Job %s already done", job)
		return
	}
	gr, err := fn(job.OrgId)
	if err != nil {
		log.Debug("Job %s: FATAL: %q", job, err)
		return
	}
	evaluator, err := NewGraphiteCheckEvaluator(gr, job.Definition)
	if err != nil {
		log.Debug("Job %s: FATAL: invalid check definition: %q", job, err)
		return
	}

	res, err := evaluator.Eval(job.LastPointTs)
	if err != nil {
		log.Debug("Job %s: FATAL: eval failed: %q", job, err)
		return
	}
	log.Debug("Job %s results: %v", job, res)
}

// execute executes an alerting job and returns any errors.
// errors are always prefixed with 'non-fatal' (i.e. error condition that imply retrying the job later might fix it)
// or 'fatal', when we're sure the job will never process successfully.
func execute(fn GraphiteReturner, job *Job, cache *lru.Cache) error {
	key := fmt.Sprintf("%d-%d", job.CheckId, job.LastPointTs.Unix())

	preConsider := time.Now()

	if time.Now().Sub(job.GeneratedAt) > time.Minute*time.Duration(10) {
		executorNumTooOld.Inc(1)
		return nil
	}

	if found, _ := cache.ContainsOrAdd(key, true); found {
		//log.Debug("T %s already done", key)
		executorNumAlreadyDone.Inc(1)
		executorConsiderJobAlreadyDone.Value(time.Since(preConsider))
		return nil
	}

	//log.Debug("T %s doing", key)
	executorNumOriginalTodo.Inc(1)
	executorConsiderJobOriginalTodo.Value(time.Since(preConsider))
	gr, err := fn(job.OrgId)
	if err != nil {
		return fmt.Errorf("fatal: job %q: %q", job, err)
	}
	if gr, ok := gr.(*graphite.GraphiteContext); ok {
		gr.AssertMinSeries = job.AssertMinSeries
		gr.AssertStart = job.AssertStart
		gr.AssertStep = job.AssertStep
		gr.AssertSteps = job.AssertSteps
	}

	preExec := time.Now()
	executorJobExecDelay.Value(preExec.Sub(job.LastPointTs))
	evaluator, err := NewGraphiteCheckEvaluator(gr, job.Definition)
	if err != nil {
		// expressions should be validated before they are stored in the db!
		return fmt.Errorf("fatal: job %q: invalid check definition %q: %q", job, job.Definition, err)
	}

	res, err := evaluator.Eval(job.LastPointTs)
	durationExec := time.Since(preExec)
	log.Debug("job results - job:%v err:%v res:%v", job, err, res)

	// the bosun api abstracts parsing, execution and graphite querying for us via 1 call.
	// we want to have some individual times
	if gr, ok := gr.(*graphite.GraphiteContext); ok {
		executorJobQueryGraphite.Value(gr.Dur)
		executorJobParseAndEval.Value(durationExec - gr.Dur)
		if gr.MissingVals > 0 {
			executorGraphiteMissingVals.Value(int64(gr.MissingVals))
		}
		if gr.EmptyResp != 0 {
			executorGraphiteEmptyResponse.Inc(int64(gr.EmptyResp))
		}
		if gr.IncompleteResp != 0 {
			executorGraphiteIncompleteResponse.Inc(int64(gr.IncompleteResp))
		}
		if gr.BadStart != 0 {
			executorGraphiteBadStart.Inc(int64(gr.BadStart))
		}
		if gr.BadStep != 0 {
			executorGraphiteBadStep.Inc(int64(gr.BadStep))
		}
		if gr.BadSteps != 0 {
			executorGraphiteBadSteps.Inc(int64(gr.BadSteps))
		}
	}

	if err != nil {
		executorAlertOutcomesErr.Inc(1)
		return fmt.Errorf("fatal: eval failed for job %q : %s", job, err.Error())
	}

	checkState := m.CheckState{
		Id:      job.CheckId,
		State:   res,
		Updated: job.LastPointTs, // this protects against jobs running out of order.
		Checked: preExec,
	}
	var affected int64
	if affected, err = sqlstore.UpdateCheckState(&checkState); err != nil {
		//check if we failed due to deadlock.
		if err.Error() == "Error 1213: Deadlock found when trying to get lock; try restarting transaction" {
			affected, err = sqlstore.UpdateCheckState(&checkState)
		}
	}
	if err != nil {
		return fmt.Errorf("non-fatal: failed to update check state: %q", err)
	}
	if gr, ok := gr.(*graphite.GraphiteContext); ok {
		requests := ""
		for _, trace := range gr.Traces {
			r := trace.Request
			// mangle trace.Response to keep the dumped out graphite
			// responses from crashing logstash
			resp := bytes.Replace(trace.Response, []byte("\n"), []byte("\n> "), -1)
			requests += fmt.Sprintf("\ntargets: %s\nfrom:%s\nto:%s\nresponse:%s\n", r.Targets, r.Start, r.End, resp)
		}
		log.Debug("Job %s state_change=%t request traces: %s", job, affected > 0, requests)
	}
	if affected > 0 && res != m.EvalResultUnknown {
		//emit a state change event.
		if job.Notifications.Enabled {
			emails := strings.Split(job.Notifications.Addresses, ",")
			if len(emails) < 1 {
				log.Debug("no email addresses provided. OrgId: %d monitorId: %d", job.OrgId, job.CheckId)
			} else {
				for _, email := range emails {
					log.Info("sending email. addr=%s, orgId=%d, monitorId=%d, endpointSlug=%s, state=%s", email, job.OrgId, job.CheckId, job.EndpointSlug, res.String())
				}
				sendCmd := m.SendEmailCommand{
					To:       emails,
					Template: "alerting_notification.html",
					Data: map[string]interface{}{
						"EndpointId":   job.EndpointId,
						"EndpointName": job.EndpointName,
						"EndpointSlug": job.EndpointSlug,
						"Settings":     job.Settings,
						"CheckType":    job.CheckType,
						"State":        res.String(),
						"TimeLastData": job.LastPointTs, // timestamp of the most recent data used
						"TimeExec":     preExec,         // when we executed the alerting rule and made the determination
					},
				}

				if err := notifications.SendEmail(&sendCmd); err != nil {
					log.Error(3, "failed to send email to %s. OrgId: %d monitorId: %d due to: %s", emails, job.OrgId, job.CheckId, err)
				}
			}
		}
	}
	//store the result in graphite.
	job.StoreResult(res)

	switch res {
	case m.EvalResultOK:
		executorAlertOutcomesOk.Inc(1)
	case m.EvalResultWarn:
		executorAlertOutcomesWarn.Inc(1)
	case m.EvalResultCrit:
		executorAlertOutcomesCrit.Inc(1)
	case m.EvalResultUnknown:
		executorAlertOutcomesUnkn.Inc(1)
	}

	return nil
}
