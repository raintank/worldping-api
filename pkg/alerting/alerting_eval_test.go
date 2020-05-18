package alerting

import (
	"encoding/json"
	"fmt"
	"testing"

	"bosun.org/graphite"
	m "github.com/raintank/worldping-api/pkg/models"
	. "github.com/smartystreets/goconvey/convey"
)

func getSeries(vals []int) graphite.Series {
	s := graphite.Series{
		Target:     "test",
		Datapoints: make([]graphite.DataPoint, len(vals)),
	}
	for i, v := range vals {
		s.Datapoints[i] = []json.Number{json.Number(fmt.Sprintf("%d", v)), json.Number(fmt.Sprintf("%d", i))}
	}
	return s
}

func check(series []graphite.Series, steps, numProbes int) m.CheckEvalResult {
	res := graphite.Response(series)
	healthSettings := m.CheckHealthSettings{
		NumProbes: numProbes,
		Steps:     steps,
	}
	result, err := eval(res, 1, &healthSettings)
	So(err, ShouldBeNil)
	return result
}

func TestAlertingEval(t *testing.T) {
	Convey("check steps=3, numProbes=1", t, func() {
		So(check(
			[]graphite.Series{
				getSeries([]int{0, 0, 0}),
			},
			3,
			1,
		), ShouldEqual, m.EvalResultOK)

		So(check(
			[]graphite.Series{
				getSeries([]int{0, 1, 1}),
			},
			3,
			1,
		), ShouldEqual, m.EvalResultOK)

		So(check(
			[]graphite.Series{
				getSeries([]int{1, 1, 1}),
			},
			3,
			1,
		), ShouldEqual, m.EvalResultCrit)

		So(check(
			[]graphite.Series{
				getSeries([]int{1, 1}),
			},
			3,
			1,
		), ShouldEqual, m.EvalResultOK)

		So(check(
			[]graphite.Series{
				getSeries([]int{1, 1, 0, 1}),
			},
			3,
			1,
		), ShouldEqual, m.EvalResultOK)

		So(check(
			[]graphite.Series{
				getSeries([]int{1, 1, 1}),
				getSeries([]int{0, 0, 0}),
			},
			3,
			1,
		), ShouldEqual, m.EvalResultCrit)

		So(check(
			[]graphite.Series{
				getSeries([]int{1, 1, 1}),
				getSeries([]int{1, 1, 1}),
			},
			3,
			1,
		), ShouldEqual, m.EvalResultCrit)
	})

	Convey("check steps=3, numProbes=2", t, func() {
		So(check(
			[]graphite.Series{
				getSeries([]int{0, 0, 0}),
				getSeries([]int{1, 1, 1}),
			},
			3,
			2,
		), ShouldEqual, m.EvalResultOK)

		So(check(
			[]graphite.Series{
				getSeries([]int{1, 1, 1}),
				getSeries([]int{1, 1, 1}),
			},
			3,
			2,
		), ShouldEqual, m.EvalResultCrit)

		So(check(
			[]graphite.Series{
				getSeries([]int{1, 1, 1}),
				getSeries([]int{1, 1, 1}),
				getSeries([]int{1, 1, 1}),
			},
			3,
			2,
		), ShouldEqual, m.EvalResultCrit)

		So(check(
			[]graphite.Series{
				getSeries([]int{1, 0, 1}),
				getSeries([]int{1, 0, 1}),
				getSeries([]int{1, 0, 1}),
			},
			3,
			2,
		), ShouldEqual, m.EvalResultOK)

		So(check(
			[]graphite.Series{
				getSeries([]int{1, 1, 1}),
				getSeries([]int{0, 0, 0}),
				getSeries([]int{0, 1, 1}),
				getSeries([]int{1, 1, 1}),
				getSeries([]int{0, 0, 0}),
			},
			3,
			2,
		), ShouldEqual, m.EvalResultCrit)
	})
}
