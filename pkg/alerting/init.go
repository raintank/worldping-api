package alerting

import (
	"fmt"
	"time"

	"github.com/hashicorp/golang-lru"
	"github.com/raintank/met"
	"github.com/raintank/worldping-api/pkg/alerting/jobqueue"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/services"
	"github.com/raintank/worldping-api/pkg/setting"
)

var jobQueueInternalItems met.Gauge
var jobQueueInternalSize met.Gauge

var tickQueueItems met.Meter
var tickQueueSize met.Gauge
var dispatcherJobsSkippedDueToSlowJobQueueInternal met.Count
var dispatcherTicksSkippedDueToSlowTickQueue met.Count

var dispatcherGetSchedules met.Timer
var dispatcherNumGetSchedules met.Count
var dispatcherJobSchedulesSeen met.Count
var dispatcherJobsScheduled met.Count

var executorNum met.Gauge

var executorNumTooOld met.Count
var executorNumAlreadyDone met.Count
var executorNumOriginalTodo met.Count
var executorAlertOutcomesErr met.Count
var executorAlertOutcomesOk met.Count
var executorAlertOutcomesCrit met.Count
var executorAlertOutcomesUnkn met.Count
var executorGraphiteEmptyResponse met.Count

var executorJobExecDelay met.Timer
var executorJobQueryGraphite met.Timer
var executorJobParseAndEval met.Timer
var executorGraphiteMissingVals met.Meter

var metricsPublisher services.MetricsPublisher

// Init initalizes all metrics
// run this function when statsd is ready, so we can create the series
func Init(metrics met.Backend, publisher services.MetricsPublisher) {
	jobQueueInternalItems = metrics.NewGauge("alert-jobqueue-internal.items", 0)
	jobQueueInternalSize = metrics.NewGauge("alert-jobqueue-internal.size", int64(setting.Alerting.InternalJobQueueSize))

	tickQueueItems = metrics.NewMeter("alert-tickqueue.items", 0)
	tickQueueSize = metrics.NewGauge("alert-tickqueue.size", int64(setting.Alerting.TickQueueSize))
	dispatcherJobsSkippedDueToSlowJobQueueInternal = metrics.NewCount("alert-dispatcher.jobs-skipped-due-to-slow-internal-jobqueue")
	dispatcherTicksSkippedDueToSlowTickQueue = metrics.NewCount("alert-dispatcher.ticks-skipped-due-to-slow-tickqueue")

	dispatcherGetSchedules = metrics.NewTimer("alert-dispatcher.get-schedules", 0)
	dispatcherNumGetSchedules = metrics.NewCount("alert-dispatcher.num-getschedules")
	dispatcherJobSchedulesSeen = metrics.NewCount("alert-dispatcher.job-schedules-seen")
	dispatcherJobsScheduled = metrics.NewCount("alert-dispatcher.jobs-scheduled")

	executorNum = metrics.NewGauge("alert-executor.num", 0)

	executorNumTooOld = metrics.NewCount("alert-executor.too-old")
	executorNumAlreadyDone = metrics.NewCount("alert-executor.already-done")
	executorNumOriginalTodo = metrics.NewCount("alert-executor.original-todo")
	executorAlertOutcomesErr = metrics.NewCount("alert-executor.alert-outcomes.error")
	executorAlertOutcomesOk = metrics.NewCount("alert-executor.alert-outcomes.ok")
	executorAlertOutcomesCrit = metrics.NewCount("alert-executor.alert-outcomes.critical")
	executorAlertOutcomesUnkn = metrics.NewCount("alert-executor.alert-outcomes.unknown")
	executorGraphiteEmptyResponse = metrics.NewCount("alert-executor.graphite-emptyresponse")

	executorJobExecDelay = metrics.NewTimer("alert-executor.job_execution_delay", time.Duration(30)*time.Second)
	executorJobQueryGraphite = metrics.NewTimer("alert-executor.job_query_graphite", 0)
	executorGraphiteMissingVals = metrics.NewMeter("alert-executor.graphite-missingVals", 0)

	metricsPublisher = publisher
}

func Construct() {
	cache, err := lru.New(setting.Alerting.ExecutorLRUSize)
	if err != nil {
		panic(fmt.Sprintf("Can't create LRU: %s", err.Error()))
	}

	if !setting.Alerting.Distributed && (!setting.Alerting.EnableScheduler || !setting.Alerting.EnableWorker) {
		log.Fatal(3, "Alerting in standalone mode requires a scheduler and a worker (enable_scheduler = true and enabled_worker = true)")
	}

	if !setting.Alerting.EnableScheduler && !setting.Alerting.EnableWorker {
		log.Fatal(3, "Alerting requires a scheduler or a worker (enable_scheduler = true or enable_worker = true)")
	}

	jobQ := jobqueue.NewJobQueue()

	// create jobs
	if setting.Alerting.EnableScheduler {
		log.Info("Alerting starting job Dispatcher")
		go dispatchJobs(jobQ)
	}

	//worker to execute the checks.
	go ChanExecutor(jobQ.Jobs(), cache)

	InitResultHandler()
}
