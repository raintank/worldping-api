package collectoreventpublisher

import (
	"fmt"
	"time"

	"github.com/grafana/grafana/pkg/log"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/nsqio/go-nsq"
	"github.com/raintank/met"
	"github.com/raintank/raintank-metric/msg"
	"github.com/raintank/raintank-metric/schema"
)

var (
	globalProducer              *nsq.Producer
	topic                       string
	collectorEventPublisherMsgs met.Count
	enabled                     bool
)

func Init(metrics met.Backend) {
	sec := setting.Cfg.Section("collector_event_publisher")

	if !sec.Key("enabled").MustBool(false) {
		enabled = false
		return
	}
	enabled = true

	addr := sec.Key("nsqd_addr").MustString("localhost:4150")
	topic = sec.Key("topic").MustString("metrics")
	cfg := nsq.NewConfig()
	cfg.UserAgent = fmt.Sprintf("probe-ctrl")
	var err error
	globalProducer, err = nsq.NewProducer(addr, cfg)
	if err != nil {
		log.Fatal(0, "failed to initialize nsq producer.", err)
	}
	collectorEventPublisherMsgs = metrics.NewCount("collectoreventpublisher.events-published")
}

func Publish(event *schema.ProbeEvent) error {
	if !enabled {
		return nil
	}
	id := time.Now().UnixNano()
	data, err := msg.CreateProbeEventMsg(event, id, msg.FormatProbeEventMsgp)
	if err != nil {
		log.Fatal(4, "Fatal error creating event message: %s", err)
	}
	collectorEventPublisherMsgs.Inc(1)
	err = globalProducer.Publish(topic, data)
	if err != nil {
		log.Fatal(4, "can't publish to nsqd: %s", err)
	}
	log.Info("event published to NSQ %d", id)

	return nil
}
