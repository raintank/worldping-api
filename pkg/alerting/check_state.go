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
	buf := make([]*m.AlertingJob, 0)
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			if len(buf) == 0 {
				break
			}
			results, err := sqlstore.BatchUpdateCheckState(buf)
			if err != nil {
				log.Error(3, "failed to update checkStates. ", err)
				buf = buf[:0]
				break
			}
			log.Debug("updated state of %d checks in batch. %d resutled in stateChange.", len(buf), len(results))
			for _, job := range results {
				stateChanges <- job
			}

			buf = buf[:0]
		case j := <-ResultQueue:
			buf = append(buf, j)
		}
	}
}

func ProcessResult(job *m.AlertingJob) {
	ResultQueue <- job
}

func handleStateChange(c chan *m.AlertingJob) {
	for job := range c {
		log.Debug("state change: orgId=%d, monitorId=%d, endpointSlug=%s, state=%s", job.OrgId, job.CheckId, job.EndpointSlug, job.NewState.String())
		if job.Notifications.Enabled {
			emails := strings.Split(job.Notifications.Addresses, ",")
			if len(emails) < 1 {
				log.Debug("no email addresses provided. OrgId: %d monitorId: %d", job.OrgId, job.CheckId)
			} else {
				emailTo := make([]string, 0)
				for _, email := range emails {
					email := strings.TrimSpace(email)
					if email == "" {
						continue
					}
					log.Info("sending email. addr=%s, orgId=%d, monitorId=%d, endpointSlug=%s, state=%s", email, job.OrgId, job.CheckId, job.EndpointSlug, job.NewState.String())
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
						"EndpointName": job.EndpointName,
						"EndpointSlug": job.EndpointSlug,
						"Settings":     job.Settings,
						"CheckType":    job.CheckType,
						"State":        job.NewState.String(),
						"TimeLastData": job.LastPointTs, // timestamp of the most recent data used
						"TimeExec":     job.TimeExec,    // when we executed the alerting rule and made the determination
					},
				}
				go func(sendCmd *m.SendEmailCommand, job *m.AlertingJob) {
					if err := notifications.SendEmail(sendCmd); err != nil {
						log.Error(3, "failed to send email to %s. OrgId: %d monitorId: %d due to: %s", sendCmd.To, job.OrgId, job.CheckId, err)
					}
				}(&sendCmd, job)
			}
		}
	}
}
