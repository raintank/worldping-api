package alerting

import (
	"time"

	"github.com/raintank/worldping-api/pkg/alerting/jobqueue"
	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
)

// getJobs retrieves all jobs for which lastPointAt % their freq == their offset.
func getJobs(lastPointAt int64) ([]*m.AlertingJob, error) {
	checks, err := sqlstore.GetChecksForAlerts(lastPointAt)
	if err != nil {
		return nil, err
	}

	jobs := make([]*m.AlertingJob, 0)
	for i := range checks {
		check := &checks[i]
		if check.HealthSettings == nil {
			continue
		}
		if check.Frequency == 0 || check.HealthSettings.Steps == 0 || check.HealthSettings.NumProbes == 0 {
			continue
		}
		jobs = append(jobs, &m.AlertingJob{CheckForAlertDTO: check})

	}
	return jobs, nil
}

func dispatchJobs(jobQ *jobqueue.JobQueue) {
	ticker := time.NewTicker(time.Second)
	offsetTicker := time.NewTicker(time.Minute)
	newOffsetChan := make(chan int)
	offset := LoadOrSetOffset()
	log.Info("Alerting using offset %d", offset)
	next := time.Now().Unix() - int64(offset)
	for {
		select {
		case lastPointAt := <-ticker.C:
			for next <= lastPointAt.Unix()-int64(offset) {
				pre := time.Now()
				jobs, err := getJobs(next)
				next++
				dispatcherNumGetSchedules.Inc(1)
				dispatcherGetSchedules.Value(time.Since(pre))

				if err != nil {
					log.Error(0, "Alerting failed to get jobs from DB: %q", err)
					continue
				}
				log.Debug("%d jobs found for TS: %d", len(jobs), next)
				for _, job := range jobs {
					job.GeneratedAt = time.Now()
					job.LastPointTs = time.Unix(next-1, 0)
					jobQ.QueueJob(job)
					dispatcherJobsScheduled.Inc(1)
				}
			}
		case <-offsetTicker.C:
			// run this in a separate goroutine so we dont block the scheduler.
			go func() {
				newOffset := LoadOrSetOffset()
				if newOffset != offset {
					newOffsetChan <- newOffset
				}
			}()
		case newOffset := <-newOffsetChan:
			log.Info("Alerting offset updated to %d", offset)
			offset = newOffset
		}
	}
}
