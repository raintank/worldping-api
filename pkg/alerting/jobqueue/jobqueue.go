package jobqueue

import (
	"time"

	"github.com/raintank/met"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
)

var (
	jobQueueInItems      met.Gauge
	jobQueueOutItems     met.Gauge
	jobQueueSize         met.Gauge
	jobsDroppedCount     met.Count
	jobsConsumedCount    met.Count
	consumerMessageDelay met.Timer
)

func InitMetrics(metrics met.Backend) {
	jobQueueInItems = metrics.NewGauge("alert-jobqueue.in-items", 0)
	jobQueueOutItems = metrics.NewGauge("alert-jobqueue.out-items", 0)
	jobQueueSize = metrics.NewGauge("alert-jobqueue.size", int64(setting.Alerting.InternalJobQueueSize))
	jobsDroppedCount = metrics.NewCount("kafka-pubsub.jobs-dropped")
	jobsConsumedCount = metrics.NewCount("kafka-pubsub.jobs-consumed")
	consumerMessageDelay = metrics.NewTimer("kafka-pubsub.message_delay", 0)
}

type JobQueue struct {
	jobsIn  chan *m.AlertingJob
	jobsOut chan *m.AlertingJob
	pubSub  *KafkaPubSub
}

func NewJobQueue() *JobQueue {
	q := new(JobQueue)
	if setting.Alerting.Distributed {
		in := make(chan *m.AlertingJob, setting.Alerting.InternalJobQueueSize)
		out := make(chan *m.AlertingJob, setting.Alerting.InternalJobQueueSize)
		pubSub := NewKafkaPubSub(setting.Kafka.Brokers, setting.Alerting.Topic, in, out)
		pubSub.Run()
		q.pubSub = pubSub
		q.jobsIn = in
		q.jobsOut = out
	} else {
		jobCh := make(chan *m.AlertingJob, setting.Alerting.InternalJobQueueSize)
		q.jobsIn = jobCh
		q.jobsOut = jobCh
	}
	go q.stats()
	return q
}

func (q *JobQueue) stats() {
	ticker := time.NewTicker(time.Second * 2)
	for range ticker.C {
		jobQueueInItems.Value(int64(len(q.jobsIn)))
		jobQueueOutItems.Value(int64(len(q.jobsOut)))
		jobQueueSize.Value(int64(setting.Alerting.InternalJobQueueSize))
	}
}

func (q *JobQueue) QueueJob(job *m.AlertingJob) {
	q.jobsIn <- job
}

func (q *JobQueue) Jobs() <-chan *m.AlertingJob {
	return q.jobsOut
}

func (q *JobQueue) Close() {
	close(q.jobsIn)
	if q.pubSub != nil {
		q.pubSub.Close()
	}
}
