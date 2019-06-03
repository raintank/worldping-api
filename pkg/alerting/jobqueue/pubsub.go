package jobqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
)

type KafkaPubSub struct {
	instance string
	consumer sarama.ConsumerGroup
	producer sarama.SyncProducer
	topic    string
	pub      <-chan *m.AlertingJob
	sub      chan<- *m.AlertingJob
	wg       sync.WaitGroup
	shutdown chan struct{}
}

func NewKafkaPubSub(brokersStr, topic string, pub <-chan *m.AlertingJob, sub chan<- *m.AlertingJob) *KafkaPubSub {
	brokers := strings.Split(brokersStr, ",")
	config := sarama.NewConfig()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.ClientID = setting.InstanceId + "_alerting"
	config.Version = sarama.V2_0_0_0
	config.Producer.Flush.Frequency = time.Millisecond * time.Duration(100)
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 3                    // Retry up to 3 times to produce the message
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Return.Successes = true
	config.Producer.Partitioner = sarama.NewHashPartitioner
	err := config.Validate()
	if err != nil {
		log.Fatal(2, "JobQueue: invalid kafka config: %s", err)
	}
	var consumer sarama.ConsumerGroup
	if setting.Alerting.EnableWorker {
		consumer, err = sarama.NewConsumerGroup(brokers, "worldping-alerts", config)
		if err != nil {
			log.Fatal(4, "JobQueue: failed to initialize kafka consumer: %s", err)
		}
	}

	producer, err := sarama.NewSyncProducer(brokers, config)
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
	if ps.consumer != nil {
		go func() {
			log.Info("JobQueue: consuming from consumerGroup")
			for {
				select {
				case <-ps.shutdown:
					log.Info("JobQueue: consumer is done")
					return
				default:
					err := ps.consumer.Consume(context.Background(), []string{ps.topic}, ps)
					if err != nil {
						log.Fatal(2, "JobQueue: comsumerGroup error: %v", err)
					}
				}
			}
		}()
	}
	go ps.produce(ps.pub)
}

func (ps *KafkaPubSub) Close() {
	close(ps.shutdown)
	if ps.consumer != nil {
		ps.consumer.Close()
	}
	ps.wg.Wait()
}

func (ps *KafkaPubSub) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (ps *KafkaPubSub) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (ps *KafkaPubSub) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ps.wg.Add(1)
	defer ps.wg.Done()

	topic := claim.Topic()
	partition := claim.Partition()
	log.Info("JobQueue: consumerClaim acquired for %d on topic %s", partition, topic)
	defer func() {
		log.Info("JobQueue: consumerClaim released partition %d on topic %s", partition, topic)
	}()
	msgCh := claim.Messages()

	for msg := range msgCh {
		log.Debug("JobQueue: kafka consumer received message: Topic %s, Partition: %d, Offset: %d, Key: %s", msg.Topic, msg.Partition, msg.Offset, msg.Key)
		consumerMessageDelay.Value(time.Since(msg.Timestamp))
		job := new(m.AlertingJob)
		err := json.Unmarshal(msg.Value, job)
		if err != nil {
			log.Error(3, "JobQueue: kafka consumer failed to unmarshal job. %s", err)
		} else {
			select {
			case ps.sub <- job:
				jobsConsumedCount.Inc(1)
			default:
				jobsDroppedCount.Inc(1)
				log.Error(3, "JobQueue: message dropped as sub chan is full")
			}
		}
		sess.MarkMessage(msg, "")
	}
	return nil
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
				Topic:     ps.topic,
				Value:     sarama.ByteEncoder(data),
				Key:       sarama.StringEncoder(fmt.Sprintf("%d-%s", job.Id, job.LastPointTs.String())),
				Timestamp: time.Now(),
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
	jobsPublishedCount.Inc(1)
}
