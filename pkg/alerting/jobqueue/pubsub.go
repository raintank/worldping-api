package jobqueue

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/bsm/sarama-cluster"
	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
)

type KafkaPubSub struct {
	instance string
	consumer *cluster.Consumer
	producer sarama.SyncProducer
	topic    string
	pub      <-chan *m.AlertingJob
	sub      chan<- *m.AlertingJob
	wg       sync.WaitGroup
	shutdown chan struct{}
}

func NewKafkaPubSub(brokersStr, topic string, pub <-chan *m.AlertingJob, sub chan<- *m.AlertingJob) *KafkaPubSub {
	brokers := strings.Split(brokersStr, ",")
	config := cluster.NewConfig()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Group.Return.Notifications = true
	config.ClientID = setting.InstanceId + "_alerting"
	config.Version = sarama.V0_10_0_0
	config.Producer.Flush.Frequency = time.Millisecond * time.Duration(100)
	config.Producer.RequiredAcks = sarama.WaitForLocal // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                     // Retry up to 10 times to produce the message
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Return.Successes = true
	err := config.Validate()
	if err != nil {
		log.Fatal(2, "JobQueue: invalid kafka consumer config: %s", err)
	}

	consumer, err := cluster.NewConsumer(brokers, "worldping-alerts", []string{topic}, config)
	if err != nil {
		log.Fatal(4, "JobQueue: failed to initialize kafka consumer: %s", err)
	}

	producer, err := sarama.NewSyncProducer(brokers, &config.Config)
	if err != nil {
		log.Fatal(4, "JobQueue: failed to initialize kafka producer: %s", err)
	}

	pubSub := &KafkaPubSub{
		instance: setting.InstanceId,
		consumer: consumer,
		producer: producer,
		topic:    topic,
		pub:      pub,
		sub:      sub,
		shutdown: make(chan struct{}),
	}
	return pubSub
}

func (ps *KafkaPubSub) Run() {
	go ps.consume(ps.sub)
	go ps.produce(ps.pub)
}

func (ps *KafkaPubSub) Close() {
	close(ps.shutdown)
	ps.wg.Wait()
}

func (ps *KafkaPubSub) consume(sub chan<- *m.AlertingJob) {
	ps.wg.Add(1)
	defer ps.wg.Done()
	log.Info("JobQueue: consuming from %s", ps.topic)

	for {
		select {
		case msg, ok := <-ps.consumer.Messages():
			if !ok {
				continue
			}
			log.Debug("JobQueue: kafka consumer received message: Topic %s, Partition: %d, Offset: %d, Key: %s", msg.Topic, msg.Partition, msg.Offset, msg.Key)
			job := new(m.AlertingJob)
			err := json.Unmarshal(msg.Value, job)
			if err != nil {
				log.Error(3, "JobQueue: kafka consumer failed to unmarshal job. %s", err)
			} else {
				sub <- job
			}
			ps.consumer.MarkOffset(msg, "")
		case notification, ok := <-ps.consumer.Notifications():
			if !ok {
				continue
			}
			claimed := ""
			for topic, partitions := range notification.Current {
				claimed += fmt.Sprintf("%s:%v ", topic, partitions)
			}
			log.Info("JobQueue: kafaka consumer rebalancing. claimed partitions: %s", claimed)
		case <-ps.shutdown:
			ps.consumer.Close()
			log.Info("JobQueue: kafka consumer for %s", ps.topic)
			return
		}
	}
}

func (ps *KafkaPubSub) produce(pub <-chan *m.AlertingJob) {
	ps.wg.Add(1)
	defer ps.wg.Done()
	done := make(chan struct{})
	for {
		select {
		case job, ok := <-pub:
			if !ok {
				log.Info("jobQueue: pub channel has closed.")
				go func() {
					ps.producer.Close()
					close(done)
				}()
				<-done
				return
			}
			data, err := json.Marshal(job)
			if err != nil {
				log.Error(3, "JobQueue: kafka producer failed to marshal job to json. %s", err)
				continue
			}
			pm := &sarama.ProducerMessage{
				Topic: ps.topic,
				Value: sarama.ByteEncoder(data),
				Key:   sarama.StringEncoder(fmt.Sprintf("%d-%s", job.Id, job.LastPointTs.String())),
			}
			go ps.sendMessage(pm)
		case <-ps.shutdown:
			go func() {
				ps.producer.Close()
				close(done)
			}()
			<-done
			return
		}
	}
}

func (ps *KafkaPubSub) sendMessage(pm *sarama.ProducerMessage) {
	for {
		partition, offset, err := ps.producer.SendMessage(pm)
		if err != nil {
			log.Error(3, "jobQueue: failed to publish message. %s", err)
			time.Sleep(time.Second)
		} else {
			log.Debug("jobQueue: message published to partition %d with offset %d", partition, offset)
			break
		}
	}
}
