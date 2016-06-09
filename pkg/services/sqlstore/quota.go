package sqlstore

import (
	"fmt"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
)

type targetCount struct {
	Count int64
}

func GetOrgQuotaByTarget(orgId int64, target string, def int64) (*m.OrgQuotaDTO, error) {
	sess, err := newSession(false, "quota")
	if err != nil {
		return nil, err
	}
	return getOrgQuotaByTarget(sess, orgId, target, def)
}

func getOrgQuotaByTarget(sess *session, orgId int64, target string, def int64) (*m.OrgQuotaDTO, error) {
	quota := m.Quota{
		Target: target,
		OrgId:  orgId,
	}
	has, err := sess.Get(&quota)
	if err != nil {
		return nil, err
	} else if !has {
		quota.Limit = def
	}

	//get quota used.
	rawSql := fmt.Sprintf("SELECT COUNT(*) as count from %s where org_id=?", dialect.Quote(target))
	var resp targetCount
	if _, err := sess.Sql(rawSql, orgId).Get(&resp); err != nil {
		return nil, err
	}

	q := &m.OrgQuotaDTO{
		Target: quota.Target,
		Limit:  quota.Limit,
		OrgId:  quota.OrgId,
		Used:   resp.Count,
	}

	return q, nil
}

func GetOrgQuotas(orgId int64) ([]m.OrgQuotaDTO, error) {
	sess, err := newSession(false, "quota")
	if err != nil {
		return nil, err
	}
	return getOrgQuotas(sess, orgId)
}

func getOrgQuotas(sess *session, orgId int64) ([]m.OrgQuotaDTO, error) {
	quotas := make([]*m.Quota, 0)
	if err := sess.Where("org_id=?", orgId).Find(&quotas); err != nil {
		return nil, err
	}

	defaultQuotas := setting.Quota.Org.ToMap()

	seenTargets := make(map[string]bool)
	for _, q := range quotas {
		seenTargets[q.Target] = true
	}

	for t, v := range defaultQuotas {
		if _, ok := seenTargets[t]; !ok {
			quotas = append(quotas, &m.Quota{
				OrgId:  orgId,
				Target: t,
				Limit:  v,
			})
		}
	}

	result := make([]m.OrgQuotaDTO, len(quotas))
	for i, q := range quotas {
		//get quota used.
		rawSql := fmt.Sprintf("SELECT COUNT(*) as count from %s where org_id=?", dialect.Quote(q.Target))
		var resp targetCount
		if _, err := sess.Sql(rawSql, q.OrgId).Get(&resp); err != nil {
			return nil, err
		}
		result[i] = m.OrgQuotaDTO{
			Target: q.Target,
			Limit:  q.Limit,
			OrgId:  q.OrgId,
			Used:   resp.Count,
		}
	}
	return result, nil
}

func UpdateOrgQuota(q *m.OrgQuotaDTO) error {
	sess, err := newSession(true, "quota")
	if err != nil {
		return err
	}
	defer sess.Cleanup()

	if err = updateOrgQuota(sess, q); err != nil {
		return err
	}
	// audit log?

	sess.Complete()
	return nil
}

func updateOrgQuota(sess *session, q *m.OrgQuotaDTO) error {
	//Check if quota is already defined in the DB
	quota := m.Quota{
		Target: q.Target,
		OrgId:  q.OrgId,
	}
	has, err := sess.Get(&quota)
	if err != nil {
		return err
	}
	quota.Limit = q.Limit
	if !has {
		//No quota in the DB for this target, so create a new one.
		if _, err := sess.Insert(&quota); err != nil {
			return err
		}
	} else {
		//update existing quota entry in the DB.
		if _, err := sess.Id(quota.Id).Update(&quota); err != nil {
			return err
		}
	}
	return nil
}

func GetGlobalQuotaByTarget(target string) (*m.GlobalQuotaDTO, error) {
	sess, err := newSession(false, "quota")
	if err != nil {
		return nil, err
	}
	return getGlobalQuotaByTarget(sess, target)
}

func getGlobalQuotaByTarget(sess *session, target string) (*m.GlobalQuotaDTO, error) {
	//get quota used.
	rawSql := fmt.Sprintf("SELECT COUNT(*) as count from %s", dialect.Quote(target))
	var resp targetCount
	if _, err := sess.Sql(rawSql).Get(&resp); err != nil {
		return nil, err
	}

	quota := &m.GlobalQuotaDTO{
		Target: target,
		Limit:  setting.Quota.Global.ToMap()[target],
		Used:   resp.Count,
	}

	return quota, nil
}
