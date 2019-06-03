package jobqueue

import (
	"testing"
	"time"

	"github.com/raintank/met/helper"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
	. "github.com/smartystreets/goconvey/convey"
)

func init() {
	metrics, _ := helper.New(false, "", "standard", "worldping-api", "test")
	InitMetrics(metrics)
}

func TestJobQueuePublish(t *testing.T) {
	setting.Alerting.InternalJobQueueSize = 1
	jobQ := NewJobQueue()

	jobs := jobQ.Jobs()
	Convey("When queuing job", t, func() {
		jobQ.QueueJob(&m.AlertingJob{
			CheckForAlertDTO: &m.CheckForAlertDTO{
				Id:   1,
				Slug: "test",
			},
		})
		var job *m.AlertingJob
		select {
		case job = <-jobs:
		default:
		}
		So(job, ShouldNotBeNil)
		So(job.Id, ShouldEqual, 1)
		So(job.Slug, ShouldEqual, "test")
	})
	Convey("When queue fills up adding job should block", t, func() {
		done := make(chan struct{})
		go func() {
			jobQ.QueueJob(&m.AlertingJob{
				CheckForAlertDTO: &m.CheckForAlertDTO{
					Id:   2,
					Slug: "test",
				},
			})
			jobQ.QueueJob(&m.AlertingJob{
				CheckForAlertDTO: &m.CheckForAlertDTO{
					Id:   3,
					Slug: "test",
				},
			})
			close(done)
		}()
		queued := false
		select {
		case <-time.After(time.Second):
		case <-done:
			queued = true
		}
		So(queued, ShouldBeFalse)

		count := 0
	LOOP:
		for {
			select {
			case <-jobs:
				count++
			case <-done:
				break LOOP
			}
		}
		So(count, ShouldEqual, 2)

	})

}
