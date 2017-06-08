package events

import (
	"strings"
	"sync"

	"github.com/Shopify/sarama"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/setting"
)

// message is the application type for a message.  This can contain identity,
// or a reference to the recevier chan for further demuxing.
type Message struct {
	Id      string
	Payload []byte
}

type KafkaPubSub struct {
	instance   string
	client     sarama.Client
	consumer   sarama.Consumer
	producer   sarama.AsyncProducer
	partitions []int32
	topic      string

	wg       sync.WaitGroup
	shutdown chan struct{}
}

func Run(brokersStr, topic string, pub, sub chan Message) {
	brokers := strings.Split(brokersStr, ",")
	config := sarama.NewConfig()
	config.ClientID = setting.InstanceId
	config.Version = sarama.V0_10_0_0
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Return.Successes = true
	err := config.Validate()
	if err != nil {
		log.Fatal(2, "kafka: invalid consumer config: %s", err)
	}

	client, err := sarama.NewClient(brokers, config)
	if err != nil {
		log.Fatal(4, "kafka: failed to create client. %s", err)
	}

	// validate our partitions
	partitions, err := client.Partitions(topic)
	if err != nil {
		log.Fatal(4, "kafka: failed to get paritions for topic %s: %s", topic, err.Error())
	}

	consumer, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		log.Fatal(2, "kafka: failed to initialize consumer: %s", err)
	}
	log.Info("kafka: consumer initialized without error")

	producer, err := sarama.NewAsyncProducerFromClient(client)
	if err != nil {
		log.Fatal(2, "kafka: failed to initialize producer: %s", err)
	}

	pubSub := &KafkaPubSub{
		instance:   setting.InstanceId,
		client:     client,
		consumer:   consumer,
		producer:   producer,
		partitions: partitions,
		topic:      topic,
		shutdown:   make(chan struct{}),
	}

	go pubSub.consume(sub)
	go pubSub.produce(pub)
}

func (ps *KafkaPubSub) consume(sub chan Message) {
	for _, p := range ps.partitions {
		ps.wg.Add(1)
		go ps.consumePartition(sub, p)
	}
}

func (ps *KafkaPubSub) consumePartition(sub chan Message, partition int32) {
	defer ps.wg.Done()
	pc, err := ps.consumer.ConsumePartition(ps.topic, partition, sarama.OffsetNewest)
	if err != nil {
		log.Fatal(4, "kafka: failed to start partitionConsumer for %s:%d. %s", ps.topic, partition, err)
	}
	log.Info("kafka: consuming from %s:%d", ps.topic, partition)

	messages := pc.Messages()
	for {
		select {
		case msg, ok := <-messages:
			if !ok {
				log.Info("kafka: consumer for %s:%d ended.", ps.topic, partition)
				return
			}
			log.Debug("kafka received message: Topic %s, Partition: %d, Offset: %d, Key: %s", msg.Topic, msg.Partition, msg.Offset, msg.Key)
			sub <- Message{
				Id:      string(msg.Key),
				Payload: msg.Value,
			}
		case <-ps.shutdown:
			pc.Close()
			log.Info("kafka: consumer for %s:%d ended.", ps.topic, partition)
			return
		}
	}
}

func (ps *KafkaPubSub) produce(pub chan Message) {
	input := ps.producer.Input()
	success := ps.producer.Successes()
	errors := ps.producer.Errors()
	done := make(chan struct{})
	for {
		select {
		case msg := <-pub:
			pm := &sarama.ProducerMessage{
				Topic: ps.topic,
				Value: sarama.ByteEncoder(msg.Payload),
				Key:   sarama.StringEncoder(msg.Id),
			}
			input <- pm
		case pm := <-success:
			log.Debug("kafka sent message: Topic: %s, Partition: %d, Offset: %d, key: %s", pm.Topic, pm.Partition, pm.Offset, pm.Key)
		case pe := <-errors:
			log.Error(3, "kafka failed to send message. %s: Topic: %s, Partition: %d, Offset: %d, key: %s", pe.Error(), pe.Msg.Topic, pe.Msg.Partition, pe.Msg.Offset, pe.Msg.Key)
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
