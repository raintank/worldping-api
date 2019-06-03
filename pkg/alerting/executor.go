package alerting

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"bosun.org/graphite"
	"github.com/hashicorp/golang-lru"
	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
	"gopkg.in/raintank/schema.v1"
)

func ChanExecutor(jobQueue <-chan *m.AlertingJob, cache *lru.Cache) {
	executorNum.Inc(1)
	defer executorNum.Dec(1)
	var wg sync.WaitGroup
	for j := range jobQueue {
		wg.Add(1)
		go func(job *m.AlertingJob) {
			execute(job, cache)
			wg.Done()
		}(j)
	}
	log.Info("Alerting: chanExecutor jobQueue closed")
	// dont return until all jobs have executed.
	wg.Wait()
	log.Info("Alerting: chanExecutor all pending jobs executed")
}

// execute executes an alerting job.
func execute(job *m.AlertingJob, cache *lru.Cache) {
	key := fmt.Sprintf("%d-%d", job.Id, job.LastPointTs.Unix())

	if time.Now().Sub(job.GeneratedAt) > time.Minute*time.Duration(10) {
		executorNumTooOld.Inc(1)
		return
	}

	if found, _ := cache.ContainsOrAdd(key, true); found {
		executorNumAlreadyDone.Inc(1)
		log.Debug("Alerting: skipping job which has already been seen. jobId: %s", key)
		return
	}

	executorNumExecuted.Inc(1)

	preExec := time.Now()
	executorJobExecDelay.Value(time.Since(job.LastPointTs))

	headers := make(http.Header)
	headers.Add("x-org-id", fmt.Sprintf("%d", job.OrgId))
	start := job.LastPointTs.Add(time.Duration(int64(-1)*job.Frequency*int64(job.HealthSettings.Steps)) * time.Second)
	req := graphite.Request{
		Start:   &start,
		End:     &job.LastPointTs,
		Targets: []string{fmt.Sprintf("worldping.%s.*.%s.error_state", job.Slug, strings.ToLower(job.CheckForAlertDTO.Type))},
	}
	log.Debug("Alerting: querying graphite with /render?target=%s&from=%d&until=%d", req.Targets[0], req.Start.Unix(), req.End.Unix())
	res, err := req.Query(setting.Alerting.GraphiteUrl+"render", headers)
	executorJobQueryGraphite.Value(time.Since(preExec))
	log.Debug("Alerting: job results - job:%v err:%v res:%v", job, err, res)

	if err != nil {
		executorAlertOutcomesErr.Inc(1)
		log.Error(3, "Alerting: query failed for job %q : %s", job, err.Error())
		return
	}

	newState, err := eval(res, job.HealthSettings)
	if err != nil {
		executorAlertOutcomesErr.Inc(1)
		log.Error(3, "Alerting: eval failed for job %q : %s", job, err.Error())
		return
	}
	job.NewState = newState
	job.TimeExec = preExec

	// lets only update the stateCheck value every second check, which will half the load we place on the DB.
	if job.State != job.NewState || job.TimeExec.Sub(job.StateCheck) > (time.Second*time.Duration(job.Frequency*2)) {
		ProcessResult(job)
	}

	//store the result in graphite.
	StoreResult(job)

	switch newState {
	case m.EvalResultOK:
		executorAlertOutcomesOk.Inc(1)
	case m.EvalResultCrit:
		executorAlertOutcomesCrit.Inc(1)
	case m.EvalResultUnknown:
		executorAlertOutcomesUnkn.Inc(1)
	}
}

func eval(res graphite.Response, healthSettings *m.CheckHealthSettings) (m.CheckEvalResult, error) {
	if len(res) == 0 {
		executorGraphiteEmptyResponse.Inc(1)
		return m.EvalResultUnknown, fmt.Errorf("fatal: no data returned for job")
	}
	badEndpoints := 0
	endpointsWithData := 0
	for _, ep := range res {
		curStreak := 0
		maxStreak := 0
		nonNullPoints := 0
		for _, dp := range ep.Datapoints {
			if dp[0].String() == "null" || dp[0].String() == "" {
				continue
			}
			nonNullPoints++
			val, err := dp[0].Float64()
			if err != nil {
				log.Error(3, "Alerting: failed to parse graphite response. value %s=[%s, %s] not a number. %s", ep.Target, dp[0].String(), dp[1].String(), err.Error())
				return m.EvalResultUnknown, err
			}
			if val > 0.0 {
				curStreak++
			} else {
				if curStreak > maxStreak {
					maxStreak = curStreak
				}
				curStreak = 0
			}
		}
		if nonNullPoints > 0 {
			endpointsWithData++
		}
		if curStreak > maxStreak {
			maxStreak = curStreak
		}

		if maxStreak >= healthSettings.Steps {
			badEndpoints++
		}
	}

	if endpointsWithData == 0 {
		return m.EvalResultUnknown, nil
	}

	if badEndpoints >= healthSettings.NumProbes {
		return m.EvalResultCrit, nil
	}

	return m.EvalResultOK, nil
}

func StoreResult(job *m.AlertingJob) {
	metrics := make([]*schema.MetricData, 3)
	metricNames := [3]string{"ok_state", "warn_state", "error_state"}
	for pos, state := range metricNames {
		metrics[pos] = &schema.MetricData{
			OrgId:    int(job.OrgId),
			Name:     fmt.Sprintf("health.%s.%s.%s", job.Slug, strings.ToLower(job.CheckForAlertDTO.Type), state),
			Metric:   fmt.Sprintf("health.%s.%s.%s", job.Slug, strings.ToLower(job.CheckForAlertDTO.Type), state),
			Interval: int(job.Frequency),
			Value:    0.0,
			Unit:     "state",
			Time:     job.LastPointTs.Unix(),
			Mtype:    "gauge",
			Tags:     nil,
		}
		metrics[pos].SetId()
	}
	if int(job.NewState) >= 0 {
		metrics[int(job.NewState)].Value = 1.0
	}

	metricsPublisher.Add(metrics)
}
