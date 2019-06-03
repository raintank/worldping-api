package alerting

import (
	"strings"
	"time"

	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/notifications"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
)

var (
	ResultQueue chan *m.AlertingJob
)

func InitResultHandler() {
	ResultQueue = make(chan *m.AlertingJob, 1000)

	stateChanges := make(chan *m.AlertingJob, 1000)

	go storeResults(stateChanges)
	go handleStateChange(stateChanges)

}

func storeResults(stateChanges chan *m.AlertingJob) {
	for j := range ResultQueue {
		saved := false
		attempts := 0
		for !saved && attempts < 3 {
			attempts++
			change, err := sqlstore.UpdateCheckState(j)
			if err != nil {
				log.Warn("failed to update checkState for checkId=%d. %s", j.Id, err)
				continue
			}
			saved = true
			log.Debug("updated state of checkId=%d stateChange=%v", j.Id, change)
			if change {
				stateChanges <- j
			}
			executorStateSaveDelay.Value(time.Since(j.TimeExec))
		}
		if !saved {
			log.Error(3, "failed to update checkState for checkId=%d", j.Id)
		}
	}
}

func ProcessResult(job *m.AlertingJob) {
	ResultQueue <- job
}

func handleStateChange(c chan *m.AlertingJob) {
	for job := range c {
		log.Debug("state change: orgId=%d, monitorId=%d, endpointSlug=%s, state=%s", job.OrgId, job.Id, job.Slug, job.NewState.String())
		if job.HealthSettings.Notifications.Enabled {
			emails := strings.Split(job.HealthSettings.Notifications.Addresses, ",")
			if len(emails) < 1 {
				log.Debug("no email addresses provided. OrgId: %d monitorId: %d", job.OrgId, job.Id)
			} else {
				emailTo := make([]string, 0)
				for _, email := range emails {
					email := strings.TrimSpace(email)
					if email == "" {
						continue
					}
					log.Info("sending email. addr=%s, orgId=%d, monitorId=%d, endpointSlug=%s, state=%s", email, job.OrgId, job.Id, job.Slug, job.NewState.String())
					emailTo = append(emailTo, email)
				}
				if len(emailTo) == 0 {
					continue
				}
				sendCmd := m.SendEmailCommand{
					To:       emailTo,
					Template: "alerting_notification.html",
					Data: map[string]interface{}{
						"EndpointId":   job.EndpointId,
						"EndpointName": job.Name,
						"EndpointSlug": job.Slug,
						"Settings":     job.Settings,
						"CheckType":    job.Type,
						"State":        job.NewState.String(),
						"TimeLastData": job.LastPointTs, // timestamp of the most recent data used
						"TimeExec":     job.TimeExec,    // when we executed the alerting rule and made the determination
					},
				}
				go func(sendCmd *m.SendEmailCommand, job *m.AlertingJob) {
					if err := notifications.SendEmail(sendCmd); err != nil {
						log.Error(3, "failed to send email to %s. OrgId: %d monitorId: %d due to: %s", sendCmd.To, job.OrgId, job.Id, err)
					}
				}(&sendCmd, job)
			}
		}
	}
}
