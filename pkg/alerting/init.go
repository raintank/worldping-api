package alerting

import (
	"fmt"

	"github.com/grafana/metrictank/stats"
	lru "github.com/hashicorp/golang-lru"
	"github.com/raintank/worldping-api/pkg/alerting/jobqueue"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/services"
	"github.com/raintank/worldping-api/pkg/setting"
)

var (
	tickQueueItems                                 = stats.NewMeter32("alert-tickqueue.items", true)
	tickQueueSize                                  = stats.NewGauge32("alert-tickqueue.size")
	dispatcherJobsSkippedDueToSlowJobQueueInternal = stats.NewCounterRate32("alert-dispatcher.jobs-skipped-due-to-slow-internal-jobqueue")
	dispatcherTicksSkippedDueToSlowTickQueue       = stats.NewCounterRate32("alert-dispatcher.ticks-skipped-due-to-slow-tickqueue")

	dispatcherGetSchedules    = stats.NewMeter32("alert-dispatcher.get-schedules", true)
	dispatcherNumGetSchedules = stats.NewCounterRate32("alert-dispatcher.num-getschedules")
	dispatcherJobsScheduled   = stats.NewCounterRate32("alert-dispatcher.jobs-scheduled")

	executorNum = stats.NewGauge32("alert-executor.num")

	executorNumTooOld             = stats.NewCounterRate32("alert-executor.too-old")
	executorNumAlreadyDone        = stats.NewCounterRate32("alert-executor.already-done")
	executorNumExecuted           = stats.NewCounterRate32("alert-executor.executed")
	executorAlertOutcomesErr      = stats.NewCounterRate32("alert-executor.alert-outcomes.error")
	executorAlertOutcomesOk       = stats.NewCounterRate32("alert-executor.alert-outcomes.ok")
	executorAlertOutcomesCrit     = stats.NewCounterRate32("alert-executor.alert-outcomes.critical")
	executorAlertOutcomesUnkn     = stats.NewCounterRate32("alert-executor.alert-outcomes.unknown")
	executorGraphiteEmptyResponse = stats.NewCounterRate32("alert-executor.graphite-emptyresponse")

	executorJobExecDelay        = stats.NewMeter32("alert-executor.job_execution_delay", true)
	executorStateSaveDelay      = stats.NewMeter32("alert-executor.state_save_delay", true)
	executorStateDBUpdate       = stats.NewMeter32("alert-executor.state_db_update", true)
	executorJobQueryGraphite    = stats.NewMeter32("alert-executor.job_query_graphite", true)
	executorGraphiteMissingVals = stats.NewCounterRate32("alert-executor.graphite-missingVals")

	executorEmailSent   = stats.NewCounterRate32("alert-executor.emails.sent")
	executorEmailFailed = stats.NewCounterRate32("alert-executor.emails.failed")

	metricsPublisher services.MetricsPublisher
)

// Init initializes the alerting engine.
func Init(publisher services.MetricsPublisher) {

	metricsPublisher = publisher
}

func Construct() {
	cache, err := lru.New(setting.Alerting.ExecutorLRUSize)
	if err != nil {
		panic(fmt.Sprintf("Can't create LRU: %s", err.Error()))
	}

	if !setting.Alerting.Distributed && !(setting.Alerting.EnableScheduler && setting.Alerting.EnableWorker) {
		log.Fatal(3, "Alerting in standalone mode requires a scheduler and a worker (enable_scheduler = true and enabled_worker = true)")
	}

	if !setting.Alerting.EnableScheduler && !setting.Alerting.EnableWorker {
		log.Fatal(3, "Alerting requires a scheduler or a worker (enable_scheduler = true or enable_worker = true)")
	}

	jobQ := jobqueue.NewJobQueue()

	// create jobs
	if setting.Alerting.EnableScheduler {
		log.Info("Alerting: starting job Dispatcher")
		go dispatchJobs(jobQ)
	}

	//worker to execute the checks.
	if setting.Alerting.EnableWorker {
		log.Info("Alerting: starting alert executor")
		go ChanExecutor(jobQ.Jobs(), cache)
	}

	InitResultHandler()
}
