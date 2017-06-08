package jobqueue

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
	m "github.com/raintank/worldping-api/pkg/models"
	. "github.com/smartystreets/goconvey/convey"
)

func TestPublish(t *testing.T) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	mp := mocks.NewSyncProducer(t, config)
	pubSub := &KafkaPubSub{
		instance: "test",
		producer: mp,
		topic:    "jq",
		shutdown: make(chan struct{}),
	}
	pub := make(chan *m.AlertingJob, 1)
	go pubSub.produce(pub)

	verifyChan := make(chan []byte)
	handler := func(msg []byte) error {
		verifyChan <- msg
		return nil
	}
	Convey("when publishing jobs", t, func() {
		for i := int64(1); i <= 3; i++ {
			mp.ExpectSendMessageWithCheckerFunctionAndSucceed(handler)
		}
		for i := int64(1); i <= 3; i++ {
			pub <- &m.AlertingJob{
				CheckForAlertDTO: &m.CheckForAlertDTO{
					Id:   i,
					Slug: "test",
				},
			}
			var j m.AlertingJob
			msg := <-verifyChan
			err := json.Unmarshal(msg, &j)
			So(err, ShouldBeNil)
			So(j.Id, ShouldEqual, i)
		}
	})
	Convey("when broker errors", t, func() {
		for i := int64(1); i <= 3; i++ {
			// fail i times, before succeeding
			for e := int64(0); e < i; e++ {
				mp.ExpectSendMessageAndFail(sarama.ErrBrokerNotAvailable)
			}
			mp.ExpectSendMessageWithCheckerFunctionAndSucceed(handler)
			pre := time.Now()
			pub <- &m.AlertingJob{
				CheckForAlertDTO: &m.CheckForAlertDTO{
					Id:   i,
					Slug: "test",
				},
			}
			var j m.AlertingJob
			msg := <-verifyChan
			end := time.Now()
			err := json.Unmarshal(msg, &j)
			So(err, ShouldBeNil)
			So(j.Id, ShouldEqual, i)
			So(end, ShouldHappenOnOrAfter, pre.Add(time.Second*time.Duration(i)))
		}
	})

	pubSub.Close()
}
