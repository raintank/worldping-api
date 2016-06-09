package alerting

import (
	"encoding/json"
	"strconv"

	"github.com/raintank/worldping-api/pkg/alerting/jobqueue"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/setting"
)

var (
	pubChan chan jobqueue.Message
	subChan chan jobqueue.Message
)

func InitJobQueue(jobQueue chan<- *Job) {

	if setting.Rabbitmq.Enabled {
		pubChan = make(chan jobqueue.Message, setting.PreAMQPJobQueueSize)
		// use rabbitmq for message distribution.

		//subchan is unbuffered as the consumer creates a goroutine for
		// every message recieved.
		subChan = make(chan jobqueue.Message)
		go jobqueue.Run(setting.Rabbitmq.Url, "alertingJobs", pubChan, subChan)
		go handleJobs(subChan, jobQueue)
	} else {
		pubChan = make(chan jobqueue.Message, setting.InternalJobQueueSize)
		// handle all message written to the publish chan.
		go handleJobs(pubChan, jobQueue)
	}
	return
}

func PublishJob(job *Job) error {
	body, err := json.Marshal(job)
	if err != nil {
		return err
	}
	msg := jobqueue.Message{
		RoutingKey: strconv.FormatInt(job.CheckId, 10),
		Payload:    body,
	}
	if setting.Rabbitmq.Enabled {
		jobQueuePreAMQPItems.Value(int64(len(pubChan)))
		jobQueuePreAMQPSize.Value(int64(setting.PreAMQPJobQueueSize))
	} else {
		jobQueueInternalItems.Value(int64(len(pubChan)))
		jobQueueInternalSize.Value(int64(setting.InternalJobQueueSize))
	}

	pubChan <- msg
	return nil
}

func handleJobs(c chan jobqueue.Message, jobQueue chan<- *Job) {
	for m := range c {
		go func(msg jobqueue.Message) {
			j := &Job{}
			err := json.Unmarshal(msg.Payload, j)
			if err != nil {
				log.Error(3, "unable to unmarshal Job. %s", err)
				return
			}
			jobQueue <- j
		}(m)
	}
}
