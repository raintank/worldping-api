package jobqueue

import (
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
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
	return q
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
