package sqlstore

import (
	"strconv"

	m "github.com/raintank/worldping-api/pkg/models"
)

func GetUsage() (*m.Usage, error) {
	sess, err := newSession(false, "endpoint")
	if err != nil {
		return nil, err
	}
	return getUsage(sess)
}

type usageRow struct {
	OrgId int64
	Count int64
}

func getUsage(sess *session) (*m.Usage, error) {
	usage := m.NewUsage()

	// get endpoints
	rows := make([]usageRow, 0)
	err := sess.Sql("SELECT org_id, COUNT(*) as count FROM endpoint GROUP BY org_id").Find(&rows)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		usage.Endpoints.Total += row.Count
		usage.Endpoints.PerOrg[strconv.FormatInt(row.OrgId, 10)] = row.Count
	}

	rows = rows[:0]
	err = sess.Sql("SELECT org_id, COUNT(*) as count FROM probe GROUP BY org_id").Find(&rows)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		usage.Probes.Total += row.Count
		usage.Probes.PerOrg[strconv.FormatInt(row.OrgId, 10)] = row.Count
	}

	rows = rows[:0]
	err = sess.Sql("SELECT org_id, COUNT(*) as count FROM `check` where type='http' GROUP BY org_id").Find(&rows)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		usage.Checks.Total += row.Count
		usage.Checks.HTTP.Total += row.Count
		usage.Checks.HTTP.PerOrg[strconv.FormatInt(row.OrgId, 10)] = row.Count
	}

	rows = rows[:0]
	err = sess.Sql("SELECT org_id, COUNT(*) as count FROM `check` where type='https' GROUP BY org_id").Find(&rows)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		usage.Checks.Total += row.Count
		usage.Checks.HTTPS.Total += row.Count
		usage.Checks.HTTPS.PerOrg[strconv.FormatInt(row.OrgId, 10)] = row.Count
	}

	rows = rows[:0]
	err = sess.Sql("SELECT org_id, COUNT(*) as count FROM `check` where type='ping' GROUP BY org_id").Find(&rows)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		usage.Checks.Total += row.Count
		usage.Checks.PING.Total += row.Count
		usage.Checks.PING.PerOrg[strconv.FormatInt(row.OrgId, 10)] = row.Count
	}

	rows = rows[:0]
	err = sess.Sql("SELECT org_id, COUNT(*) as count FROM `check` where type='dns' GROUP BY org_id").Find(&rows)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		usage.Checks.Total += row.Count
		usage.Checks.DNS.Total += row.Count
		usage.Checks.DNS.PerOrg[strconv.FormatInt(row.OrgId, 10)] = row.Count
	}

	return usage, nil
}
