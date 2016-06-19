package models

import (
	"fmt"
	"time"
)

// Job is a job for an alert execution
// note that LastPointTs is a time denoting the timestamp of the last point to run against
// this way the check runs always on the right data, irrespective of execution delays
// that said, for convenience, we track the generatedAt timestamp
type AlertingJob struct {
	OrgId           int64
	CheckId         int64
	EndpointId      int64
	EndpointName    string
	EndpointSlug    string
	Settings        map[string]interface{}
	CheckType       string
	Notifications   CheckNotificationSetting
	Freq            int64
	Offset          int64 // offset on top of "even" minute/10s/.. intervalst
	State           CheckEvalResult
	StateCheck      time.Time
	StateChange     time.Time
	Definition      CheckDef
	GeneratedAt     time.Time
	LastPointTs     time.Time
	AssertMinSeries int       // to verify during execution at least this many series are returned (would be nice at some point to include actual number of collectors)
	AssertStart     time.Time // to verify timestamps in response
	AssertStep      int       // to verify step duration
	AssertSteps     int       // to verify during execution this many points are included
	NewState        CheckEvalResult
	TimeExec        time.Time
}

func (job AlertingJob) String() string {
	return fmt.Sprintf("<Job> checkId=%d generatedAt=%s lastPointTs=%s definition: %s", job.CheckId, job.GeneratedAt, job.LastPointTs, job.Definition)
}

func (job *AlertingJob) SetAssertStart() {
	startTs := job.LastPointTs.Unix() - int64(job.AssertStep*(job.AssertSteps))
	job.AssertStart = time.Unix((startTs+int64(job.AssertStep))-(startTs%int64(job.AssertStep)), 0)
}

type CheckDef struct {
	CritExpr string
	WarnExpr string
}

func (c CheckDef) String() string {
	return fmt.Sprintf("<CheckDef> Crit: ''%s' -- Warn: '%s'", c.CritExpr, c.WarnExpr)
}
