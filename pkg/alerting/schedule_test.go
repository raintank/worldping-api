package alerting

import (
	"testing"
	"time"

	m "github.com/grafana/grafana/pkg/models"
	. "github.com/smartystreets/goconvey/convey"
)

func TestScheduleBuilding(t *testing.T) {

	Convey("Can build schedules from monitor configs", t, func() {
		m := &m.MonitorForAlertDTO{
			EndpointSlug:    "test_endpoint_be",
			MonitorTypeName: "smtp",
			Frequency:       60,
			Offset:          37,
			HealthSettings: &m.MonitorHealthSettingDTO{
				NumCollectors: 16,
				Steps:         5,
			},
		}
		sched := buildJobForMonitor(m)

		if sched.Freq != 60 {
			t.Errorf("sched.Freq should be 60, not %d", sched.Freq)
		}
		if sched.Offset != 37 {
			t.Errorf("sched.Offset should be 37, not %d", sched.Offset)
		}
		critExpr := `sum(t(streak(graphite("litmus.test_endpoint_be.*.smtp.error_state", "300s", "", "")) == 5 , "")) >= 16`
		if sched.Definition.CritExpr != critExpr {
			t.Errorf("sched.Definition.CritExpr should be '%s' not '%s'", critExpr, sched.Definition.CritExpr)
		}
	})
}

func TestJobAssertStart(t *testing.T) {
	type cas struct {
		step  int
		steps int
		first int64
		last  int64
	}
	cases := []cas{
		// note that graphite quantizes down, so graphite output should be points at 20, 30
		{
			10, 2, 20, 33,
		},
	}

	for i, c := range cases {
		job := &Job{
			LastPointTs: time.Unix(c.last, 0),
			AssertStep:  c.step,
			AssertSteps: c.steps,
		}
		job.assertStart()
		start := job.AssertStart.Unix()
		if start != c.first {
			t.Fatalf("job assertStart case %d expected %d, got %d", i, c.first, start)
		}
	}
}
