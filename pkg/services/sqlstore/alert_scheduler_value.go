package sqlstore

import (
	m "github.com/raintank/worldping-api/pkg/models"
)

func GetAlertSchedulerValue(id string) (string, error) {
	sess, err := newSession(false, "alert_scheduler_value")
	if err != nil {
		return "", err
	}
	return getAlertSchedulerValue(sess, id)
}

func getAlertSchedulerValue(sess *session, id string) (string, error) {
	rawSql := "SELECT value from alert_scheduler_value where id=?"
	results, err := sess.Query(rawSql, id)

	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "", nil
	}

	return string(results[0]["value"]), nil
}

func UpdateAlertSchedulerValue(id, value string) error {
	sess, err := newSession(true, "alert_scheduler_value")
	if err != nil {
		return err
	}
	defer sess.Cleanup()

	if err = updateAlertSchedulerValue(sess, id, value); err != nil {
		return err
	}
	// audit log?

	sess.Complete()
	return nil
}

func updateAlertSchedulerValue(sess *session, id, value string) error {
	entity := m.AlertSchedulerValue{
		Id:    id,
		Value: value,
	}

	affected, err := sess.Update(&entity)
	if err == nil && affected == 0 {
		_, err = sess.Insert(&entity)
	}

	return err
}
