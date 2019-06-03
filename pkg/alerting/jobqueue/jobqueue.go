package jobqueue

import (
	"time"

	"github.com/grafana/metrictank/stats"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
)

var (
	jobQueueInItems      = stats.NewGauge32("alert-jobqueue.in-items")
	jobQueueOutItems     = stats.NewGauge32("alert-jobqueue.out-items")
	jobQueueSize         = stats.NewGauge32("alert-jobqueue.size")
	jobsDroppedCount     = stats.NewCounterRate32("kafka-pubsub.jobs-dropped")
	jobsConsumedCount    = stats.NewCounterRate32("kafka-pubsub.jobs-consumed")
	jobsPublishedCount   = stats.NewCounterRate32("kafka-pubsub.jobs-published")
	consumerMessageDelay = stats.NewMeter32("kafka-pubsub.message_delay", true)
)

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
		jobQueueInItems.Set(len(q.jobsIn))
		jobQueueOutItems.Set(len(q.jobsOut))
		jobQueueSize.Set(setting.Alerting.InternalJobQueueSize)
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
