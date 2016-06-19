package alerting

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/hashicorp/golang-lru"
	"github.com/raintank/worldping-api/pkg/graphite"
	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
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

func ChanExecutor(fn GraphiteReturner, jobQueue <-chan *m.AlertingJob, cache *lru.Cache) {
	executorNum.Inc(1)
	defer executorNum.Dec(1)

	for j := range jobQueue {
		go func(job *m.AlertingJob) {
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

func inspect(fn GraphiteReturner, job *m.AlertingJob, cache *lru.Cache) {
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
func execute(fn GraphiteReturner, job *m.AlertingJob, cache *lru.Cache) error {
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
	job.NewState = res
	job.TimeExec = preExec

	// lets only update the stateCheck value every second check, which will half the load we place on the DB.
	if job.State != job.NewState || job.TimeExec.Sub(job.StateCheck) > (time.Second*time.Duration(job.Freq*2)) {
		ProcessResult(job)
	}

	//store the result in graphite.
	StoreResult(job)

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
