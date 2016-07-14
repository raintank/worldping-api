package sqlstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/raintank/worldping-api/pkg/events"
	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
)

type endpointRow struct {
	m.Endpoint    `xorm:"extends"`
	m.Check       `xorm:"extends"`
	m.EndpointTag `xorm:"extends"`
}

type endpointRows []*endpointRow

func (endpointRows) TableName() string {
	return "endpoint"
}

// scrutinizeState fixes the state.  We can't just trust what the database says, we have to verify that the value actually has been updated recently.
// we can simply do this by requiring that the value has been updated since 2*frequency ago.
func scrutinizeState(now time.Time, monitor *m.Check) {
	if monitor.State == m.EvalResultUnknown {
		return
	}
	freq := time.Duration(monitor.Frequency) * time.Second
	oldest := now.Add(-3 * freq)
	if monitor.StateCheck.Before(oldest) {
		monitor.State = m.EvalResultUnknown
		monitor.StateChange = monitor.StateCheck
	}
}

func (rows endpointRows) ToDTO() []m.EndpointDTO {
	endpointsById := make(map[int64]m.EndpointDTO)
	endpointChecksById := make(map[int64]map[int64]m.Check)
	endpointTagsById := make(map[int64]map[string]struct{})
	for _, r := range rows {
		_, ok := endpointsById[r.Endpoint.Id]

		if !ok {
			endpointsById[r.Endpoint.Id] = m.EndpointDTO{
				Id:      r.Endpoint.Id,
				OrgId:   r.Endpoint.OrgId,
				Name:    r.Endpoint.Name,
				Slug:    r.Endpoint.Slug,
				Checks:  make([]m.Check, 0),
				Tags:    make([]string, 0),
				Created: r.Endpoint.Created,
				Updated: r.Endpoint.Updated,
			}
			endpointChecksById[r.Endpoint.Id] = make(map[int64]m.Check)
			endpointTagsById[r.Endpoint.Id] = make(map[string]struct{})
			if r.Check.Id != 0 {
				endpointChecksById[r.Endpoint.Id][r.Check.Id] = r.Check
			}
			if r.EndpointTag.Tag != "" {
				endpointTagsById[r.Endpoint.Id][r.EndpointTag.Tag] = struct{}{}
			}
		} else {
			if r.Check.Id != 0 {
				_, ecOk := endpointChecksById[r.Endpoint.Id][r.Check.Id]
				if !ecOk {
					endpointChecksById[r.Endpoint.Id][r.Check.Id] = r.Check
				}
			}
			if r.EndpointTag.Tag != "" {
				_, tagOk := endpointTagsById[r.Endpoint.Id][r.EndpointTag.Tag]
				if !tagOk {
					endpointTagsById[r.Endpoint.Id][r.EndpointTag.Tag] = struct{}{}
				}
			}
		}
	}
	endpoints := make([]m.EndpointDTO, len(endpointsById))
	i := 0
	for _, e := range endpointsById {
		for _, c := range endpointChecksById[e.Id] {
			scrutinizeState(time.Now(), &c)
			e.Checks = append(e.Checks, c)
		}

		for t := range endpointTagsById[e.Id] {
			e.Tags = append(e.Tags, t)
		}

		endpoints[i] = e
		i++
	}
	return endpoints
}

func GetEndpoints(query *m.GetEndpointsQuery) ([]m.EndpointDTO, error) {
	log.Info("ENDPOINT: creating new DB session")
	sess, err := newSession(false, "endpoint")
	log.Info("ENDPOINT: session created")
	if err != nil {
		return nil, err
	}
	return getEndpoints(sess, query)
}

func getEndpoints(sess *session, query *m.GetEndpointsQuery) ([]m.EndpointDTO, error) {
	var e endpointRows
	if query.OrgId != 0 {
		sess.Where("endpoint.org_id=?", query.OrgId)
	}
	if query.Name != "" {
		sess.And("endpoint.name like ?", query.Name)
	}
	if query.Tag != "" {
		sess.Join("INNER", []string{"endpoint_tag", "et"}, "endpoint.id = et.endpoint_id").Where("et.tag=?", query.Tag)
	}
	if query.OrderBy == "" {
		query.OrderBy = "name"
	}

	sess.Asc(query.OrderBy)

	sess.Join("LEFT", "check", "endpoint.id = `check`.endpoint_id")
	sess.Join("LEFT", "endpoint_tag", "endpoint.id = endpoint_tag.endpoint_id")
	sess.Cols("`endpoint`.*", "`check`.*", "`endpoint_tag`.*")
	log.Info("ENDPOINT: executing find query")
	err := sess.Find(&e)
	if err != nil {
		return nil, err
	}
	log.Info("ENDPOINT: converting rows to endpointDTO")
	return e.ToDTO(), nil
}

func GetEndpointById(orgId, id int64) (*m.EndpointDTO, error) {
	sess, err := newSession(false, "endpoint")
	if err != nil {
		return nil, err
	}
	return getEndpointById(sess, orgId, id)
}

func getEndpointById(sess *session, orgId, id int64) (*m.EndpointDTO, error) {
	var e endpointRows
	sess.Where("endpoint.id=? AND endpoint.org_id=?", id, orgId)
	sess.Join("LEFT", "check", "endpoint.id = `check`.endpoint_id")
	sess.Join("LEFT", "endpoint_tag", "endpoint.id = endpoint_tag.endpoint_id")
	sess.Cols("`endpoint`.*", "`check`.*", "`endpoint_tag`.*")
	err := sess.Find(&e)
	if err != nil {
		return nil, err
	}
	if len(e) == 0 {
		return nil, m.NewNotFoundError("endpoint not found")
	}
	return &e.ToDTO()[0], nil
}

func AddEndpoint(e *m.EndpointDTO) error {
	sess, err := newSession(true, "endpoint")
	if err != nil {
		return err
	}
	defer sess.Cleanup()

	if err = addEndpoint(sess, e); err != nil {
		return err
	}
	sess.Complete()
	return nil
}

func addEndpoint(sess *session, e *m.EndpointDTO) error {
	endpoint := &m.Endpoint{
		OrgId:   e.OrgId,
		Name:    e.Name,
		Created: time.Now(),
		Updated: time.Now(),
	}
	endpoint.UpdateSlug()
	if _, err := sess.Insert(endpoint); err != nil {
		return err
	}
	e.Id = endpoint.Id
	e.Created = endpoint.Created
	e.Updated = endpoint.Updated
	e.Slug = endpoint.Slug

	endpointTags := make([]m.EndpointTag, 0, len(e.Tags))
	for _, tag := range e.Tags {
		endpointTags = append(endpointTags, m.EndpointTag{
			OrgId:      e.OrgId,
			EndpointId: endpoint.Id,
			Tag:        tag,
			Created:    time.Now(),
		})
	}
	if len(endpointTags) > 0 {
		sess.Table("endpoint_tag")
		if _, err := sess.Insert(&endpointTags); err != nil {
			return err
		}
	}

	//perform each insert on its own so that the ID field gets assigned and task created
	for i := range e.Checks {
		c := &e.Checks[i]
		c.OrgId = e.OrgId
		c.EndpointId = e.Id
		if err := addCheck(sess, c); err != nil {
			return err
		}
	}

	events.Publish(&events.EndpointCreated{
		Ts:      e.Created,
		Payload: e,
	}, 0)
	return nil
}

func UpdateEndpoint(e *m.EndpointDTO) error {
	sess, err := newSession(true, "endpoint")
	if err != nil {
		return err
	}
	defer sess.Cleanup()

	if err = updateEndpoint(sess, e); err != nil {
		return err
	}
	sess.Complete()
	return nil
}

func updateEndpoint(sess *session, e *m.EndpointDTO) error {
	existing, err := getEndpointById(sess, e.OrgId, e.Id)
	if err != nil {
		return err
	}
	if existing == nil {
		return m.NewNotFoundError("endpoint not found")
	}
	endpoint := &m.Endpoint{
		Id:      e.Id,
		OrgId:   e.OrgId,
		Name:    e.Name,
		Created: existing.Created,
		Updated: time.Now(),
	}
	endpoint.UpdateSlug()
	if _, err := sess.Id(endpoint.Id).Update(endpoint); err != nil {
		return err
	}

	e.Slug = endpoint.Slug
	e.Updated = endpoint.Updated

	/***** Update Tags **********/

	tagMap := make(map[string]bool)
	tagsToDelete := make([]string, 0)
	tagsToAddMap := make(map[string]bool, 0)
	// create map of current tags
	for _, t := range existing.Tags {
		tagMap[t] = false
	}

	// create map of tags to add. We use a map
	// to ensure that we only add each tag once.
	for _, t := range e.Tags {
		if _, ok := tagMap[t]; !ok {
			tagsToAddMap[t] = true
		}
		// mark that this tag has been seen.
		tagMap[t] = true
	}

	//create list of tags to delete
	for t, seen := range tagMap {
		if !seen {
			tagsToDelete = append(tagsToDelete, t)
		}
	}

	// create list of tags to add.
	tagsToAdd := make([]string, len(tagsToAddMap))
	i := 0
	for t := range tagsToAddMap {
		tagsToAdd[i] = t
		i += 1
	}
	if len(tagsToDelete) > 0 {
		sess.Table("endpoint_tag")
		sess.Where("endpoint_id=? AND org_id=?", e.Id, e.OrgId)
		sess.In("tag", tagsToDelete)
		if _, err := sess.Delete(&m.EndpointTag{}); err != nil {
			return err
		}
	}
	if len(tagsToAdd) > 0 {
		newEndpointTags := make([]m.EndpointTag, len(tagsToAdd))
		for i, tag := range tagsToAdd {
			newEndpointTags[i] = m.EndpointTag{
				OrgId:      e.OrgId,
				EndpointId: e.Id,
				Tag:        tag,
				Created:    time.Now(),
			}
		}
		sess.Table("endpoint_tag")
		if _, err := sess.Insert(&newEndpointTags); err != nil {
			return err
		}
	}

	/***** Update Checks **********/

	checkUpdates := make([]*m.Check, 0)
	checkAdds := make([]*m.Check, 0)
	checkDeletes := make([]*m.Check, 0)

	checkMap := make(map[m.CheckType]*m.Check)
	seenChecks := make(map[m.CheckType]bool)
	for i := range existing.Checks {
		c := &existing.Checks[i]
		checkMap[c.Type] = c
	}
	for i := range e.Checks {
		c := &e.Checks[i]
		c.EndpointId = e.Id
		c.OrgId = e.OrgId
		seenChecks[c.Type] = true
		ec, ok := checkMap[c.Type]
		if !ok {
			checkAdds = append(checkAdds, c)
		} else if c.Id == ec.Id {
			cjson, err := json.Marshal(c)
			if err != nil {
				return err
			}
			ecjson, err := json.Marshal(ec)
			if !bytes.Equal(ecjson, cjson) || existing.Slug != endpoint.Slug {
				c.Created = ec.Created
				checkUpdates = append(checkUpdates, c)
			}
		} else {
			return m.NewValidationError(fmt.Sprintf("an existing %s check is already defined for this endpoint.", c.Type))
		}
	}
	for t, ec := range checkMap {
		if _, ok := seenChecks[t]; !ok {
			checkDeletes = append(checkDeletes, ec)
		}
	}

	for _, c := range checkDeletes {
		if err := deleteCheck(sess, c); err != nil {
			return err
		}
	}

	for _, c := range checkAdds {
		if err := addCheck(sess, c); err != nil {
			return err
		}
	}

	for _, c := range checkUpdates {
		if err := updateCheck(sess, c); err != nil {
			return err
		}
	}

	evnt := new(events.EndpointUpdated)
	evnt.Ts = e.Updated
	evnt.Payload.Current = e
	evnt.Payload.Last = existing
	events.Publish(evnt, 0)

	return nil
}

func DeleteEndpoint(orgId, id int64) error {
	sess, err := newSession(true, "endpoint")
	if err != nil {
		return err
	}
	defer sess.Cleanup()

	if err = deleteEndpoint(sess, orgId, id); err != nil {
		return err
	}
	sess.Complete()
	return nil
}

func deleteEndpoint(sess *session, orgId, id int64) error {
	existing, err := getEndpointById(sess, orgId, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return m.NewNotFoundError("endpoint not found")
	}
	var rawSql = "DELETE FROM endpoint WHERE id=? and org_id=?"
	_, err = sess.Exec(rawSql, id, orgId)
	if err != nil {
		return err
	}

	rawSql = "DELETE FROM endpoint_tag WHERE endpoint_id=? and org_id=?"
	if _, err := sess.Exec(rawSql, id, orgId); err != nil {
		return err
	}

	for _, c := range existing.Checks {
		if err := deleteCheck(sess, &c); err != nil {
			return err
		}
	}
	events.Publish(&events.EndpointDeleted{
		Ts:      time.Now(),
		Payload: existing,
	}, 0)
	return nil
}

func addCheck(sess *session, c *m.Check) error {
	c.State = -1
	c.StateCheck = time.Now()
	c.StateChange = time.Now()
	c.Offset = c.EndpointId % c.Frequency
	c.Created = time.Now()
	c.Updated = time.Now()
	sess.Table("check")
	sess.UseBool("enabled")
	if _, err := sess.Insert(c); err != nil {
		return err
	}

	return addCheckRoutes(sess, c)
}

func addCheckRoutes(sess *session, c *m.Check) error {
	switch c.Route.Type {
	case m.RouteByTags:
		tagRoutes := make([]m.RouteByTagIndex, len(c.Route.Config["tags"].([]string)))
		for i, tag := range c.Route.Config["tags"].([]string) {
			tagRoutes[i] = m.RouteByTagIndex{
				CheckId: c.Id,
				Tag:     tag,
				Created: time.Now(),
				OrgId:   c.OrgId,
			}
		}
		if _, err := sess.Insert(&tagRoutes); err != nil {
			return err
		}
	case m.RouteByIds:
		idxs := make([]m.RouteByIdIndex, len(c.Route.Config["ids"].([]int64)))
		for i, id := range c.Route.Config["ids"].([]int64) {
			idxs[i] = m.RouteByIdIndex{
				CheckId: c.Id,
				ProbeId: id,
				Created: time.Now(),
			}
		}
		if _, err := sess.Insert(&idxs); err != nil {
			return err
		}
	default:
		return m.UnknownRouteType
	}
	return nil
}

func deleteCheckRoutes(sess *session, c *m.Check) error {
	deletes := []string{
		"DELETE from route_by_id_index where check_id = ?",
		"DELETE from route_by_tag_index where check_id = ?",
	}
	for _, sql := range deletes {
		_, err := sess.Exec(sql, c.Id)
		if err != nil {
			return err
		}
	}
	return nil
}

func updateCheck(sess *session, c *m.Check) error {
	existing, err := getCheckById(sess, c.OrgId, c.Id)
	if err != nil {
		return err
	}
	c.Updated = time.Now()
	c.Offset = c.EndpointId % c.Frequency
	sess.Table("check")
	sess.UseBool("enabled")
	if !c.Enabled && existing.Enabled {
		c.StateChange = time.Now()
		c.State = -1
	}
	_, err = sess.Id(c.Id).Update(c)
	if err != nil {
		return err
	}

	// handle task routes.
	if existing.Route.Type != c.Route.Type {
		if err := deleteCheckRoutes(sess, existing); err != nil {
			return err
		}
		if err := addCheckRoutes(sess, c); err != nil {
			return err
		}
	} else {
		switch c.Route.Type {
		case m.RouteByTags:
			existingTags := make(map[string]struct{})
			tagsToAdd := make([]string, 0)
			tagsToDel := make([]string, 0)
			currentTags := make(map[string]struct{})

			for _, tag := range existing.Route.Config["tags"].([]string) {
				existingTags[tag] = struct{}{}
			}
			for _, tag := range c.Route.Config["tags"].([]string) {
				currentTags[tag] = struct{}{}
				if _, ok := existingTags[tag]; !ok {
					tagsToAdd = append(tagsToAdd, tag)
				}
			}
			for tag := range existingTags {
				if _, ok := currentTags[tag]; !ok {
					tagsToDel = append(tagsToDel, tag)
				}
			}
			if len(tagsToDel) > 0 {
				for _, tag := range tagsToDel {
					_, err := sess.Where("check_id=? AND tag=?", c.Id, tag).Delete(&m.RouteByTagIndex{})
					if err != nil {
						return err
					}
				}
			}
			if len(tagsToAdd) > 0 {
				tagRoutes := make([]m.RouteByTagIndex, len(tagsToAdd))
				for i, tag := range tagsToAdd {
					tagRoutes[i] = m.RouteByTagIndex{
						CheckId: c.Id,
						Tag:     tag,
						Created: time.Now(),
					}
				}
				_, err := sess.Insert(&tagRoutes)
				if err != nil {
					return err
				}
			}
		case m.RouteByIds:
			existingIds := make(map[int64]struct{})
			idsToAdd := make([]int64, 0)
			idsToDel := make([]int64, 0)
			currentIds := make(map[int64]struct{})

			for _, id := range existing.Route.Config["ids"].([]int64) {
				existingIds[id] = struct{}{}
			}
			for _, id := range c.Route.Config["ids"].([]int64) {
				currentIds[id] = struct{}{}
				if _, ok := existingIds[id]; !ok {
					idsToAdd = append(idsToAdd, id)
				}
			}
			for id := range existingIds {
				if _, ok := currentIds[id]; !ok {
					idsToDel = append(idsToDel, id)
				}
			}
			if len(idsToDel) > 0 {
				for _, id := range idsToDel {
					_, err := sess.Where("check_id=? and probe_id=?", c.Id, id).Delete(&m.RouteByIdIndex{})
					if err != nil {
						return err
					}
				}
			}
			if len(idsToAdd) > 0 {
				idRoutes := make([]m.RouteByIdIndex, len(idsToAdd))
				for i, id := range idsToAdd {
					idRoutes[i] = m.RouteByIdIndex{
						CheckId: c.Id,
						ProbeId: id,
						Created: time.Now(),
					}
				}
				_, err := sess.Insert(&idRoutes)
				if err != nil {
					return err
				}
			}
		default:
			return m.NewValidationError(m.UnknownRouteType.Error())
		}
	}
	return err
}

func GetCheckById(orgId, checkId int64) (*m.Check, error) {
	sess, err := newSession(false, "check")
	if err != nil {
		return nil, err
	}
	return getCheckById(sess, orgId, checkId)
}

func getCheckById(sess *session, orgId, checkId int64) (*m.Check, error) {
	sess.Where("org_id=? AND id=?", orgId, checkId)
	check := &m.Check{}
	has, err := sess.Get(check)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, m.NewNotFoundError("check not found")
	}
	return check, nil
}

func deleteCheck(sess *session, c *m.Check) error {
	sess.Table("check")
	if _, err := sess.Id(c.Id).Delete(&m.Check{}); err != nil {
		return err
	}

	return deleteCheckRoutes(sess, c)
}

func GetEndpointTags(orgId int64) ([]string, error) {
	sess, err := newSession(false, "endpoint_tag")
	if err != nil {
		return nil, err
	}
	return getEndpointTags(sess, orgId)
}

func getEndpointTags(sess *session, orgId int64) ([]string, error) {
	type tagRow struct {
		Tag string
	}
	rawSql := `SELECT DISTINCT(tag) as tag FROM endpoint_tag WHERE org_id=?`

	sess.Sql(rawSql, orgId)
	tags := make([]tagRow, 0)

	if err := sess.Find(&tags); err != nil {
		return nil, err
	}

	result := make([]string, 0)
	for _, row := range tags {
		result = append(result, row.Tag)
	}

	return result, nil
}

func GetProbeChecks(probe *m.ProbeDTO) ([]m.Check, error) {
	sess, err := newSession(false, "check")
	if err != nil {
		return nil, err
	}
	return getProbeChecks(sess, probe)
}

func getProbeChecks(sess *session, probe *m.ProbeDTO) ([]m.Check, error) {
	checks := make([]m.Check, 0)

	type checkIdRow struct {
		CheckId int64
	}
	checkIds := make([]checkIdRow, 0)
	rawQuery := "SELECT check_id FROM route_by_id_index where probe_id = ?"
	rawParams := make([]interface{}, 0)
	rawParams = append(rawParams, probe.Id)

	q := `SELECT DISTINCT(idx.check_id)
		FROM route_by_tag_index as idx
		INNER JOIN probe_tag on idx.org_id=probe_tag.org_id and idx.tag = probe_tag.tag
		WHERE probe_tag.probe_id=?`
	rawParams = append(rawParams, probe.Id)
	rawQuery = fmt.Sprintf("%s UNION %s", rawQuery, q)

	err := sess.Sql(rawQuery, rawParams...).Find(&checkIds)
	if err != nil {
		return nil, err
	}

	if len(checkIds) == 0 {
		return checks, nil
	}
	cid := make([]int64, len(checkIds))
	for i, c := range checkIds {
		cid[i] = c.CheckId
	}
	sess.Table("check")
	sess.In("id", cid).And("`check`.enabled=1")
	err = sess.Find(&checks)
	return checks, err
}

func GetProbeChecksWithEndpointSlug(probe *m.ProbeDTO) ([]m.CheckWithSlug, error) {
	sess, err := newSession(false, "check")
	if err != nil {
		return nil, err
	}
	return getProbeChecksWithEndpointSlug(sess, probe)
}

func getProbeChecksWithEndpointSlug(sess *session, probe *m.ProbeDTO) ([]m.CheckWithSlug, error) {
	checks := make([]m.CheckWithSlug, 0)

	type checkIdRow struct {
		CheckId int64
	}
	checkIds := make([]checkIdRow, 0)
	rawQuery := "SELECT check_id FROM route_by_id_index where probe_id = ?"
	rawParams := make([]interface{}, 0)
	rawParams = append(rawParams, probe.Id)

	q := `SELECT DISTINCT(idx.check_id)
		FROM route_by_tag_index as idx
		INNER JOIN probe_tag on idx.org_id=probe_tag.org_id and idx.tag = probe_tag.tag
		WHERE probe_tag.probe_id=?`
	rawParams = append(rawParams, probe.Id)
	rawQuery = fmt.Sprintf("%s UNION %s", rawQuery, q)

	err := sess.Sql(rawQuery, rawParams...).Find(&checkIds)
	if err != nil {
		return nil, err
	}

	if len(checkIds) == 0 {
		return checks, nil
	}
	cid := make([]int64, len(checkIds))
	for i, c := range checkIds {
		cid[i] = c.CheckId
	}
	sess.Table("check")
	sess.Join("INNER", "endpoint", "`check`.endpoint_id=endpoint.id")
	sess.In("`check`.id", cid).And("`check`.enabled=1")
	sess.Cols("`check`.*", "endpoint.slug")
	err = sess.Find(&checks)
	return checks, err
}

func BatchUpdateCheckState(jobs []*m.AlertingJob) ([]*m.AlertingJob, error) {
	sess, err := newSession(true, "check")
	if err != nil {
		return nil, err
	}
	defer sess.Cleanup()
	jobsWithStateChange, err := batchUpdateCheckState(sess, jobs)
	if err != nil {
		return nil, err
	}
	sess.Complete()
	return jobsWithStateChange, nil
}

func batchUpdateCheckState(sess *session, jobs []*m.AlertingJob) ([]*m.AlertingJob, error) {
	stateSql := "UPDATE `check` SET state=?, state_change=? WHERE id=? AND state != ? AND state_change < ?"
	lastCheckSql := "UPDATE `check` SET state_check=? WHERE id=?"
	jobsWithStateChange := make([]*m.AlertingJob, 0)
	for _, j := range jobs {
		res, err := sess.Exec(stateSql, int(j.NewState), j.TimeExec, j.CheckId, int(j.NewState), j.TimeExec)
		if err != nil {
			return nil, err
		}

		aff, _ := res.RowsAffected()
		if aff > 0 {
			// state change.
			jobsWithStateChange = append(jobsWithStateChange, j)
		}

		res, err = sess.Exec(lastCheckSql, j.TimeExec, j.CheckId)
		if err != nil {
			return nil, err
		}
	}

	return jobsWithStateChange, nil
}

func GetChecksForAlerts(ts int64) ([]m.CheckForAlertDTO, error) {
	sess, err := newSession(false, "check")
	if err != nil {
		return nil, err
	}
	return getChecksForAlerts(sess, ts)
}

func getChecksForAlerts(sess *session, ts int64) ([]m.CheckForAlertDTO, error) {
	sess.Join("INNER", "endpoint", "check.endpoint_id=endpoint.id")
	sess.Where("`check`.enabled=1 AND (? % `check`.frequency) = `check`.offset", ts)
	sess.Cols(
		"`check`.id",
		"`check`.org_id",
		"`check`.endpoint_id",
		"endpoint.slug",
		"endpoint.name",
		"`check`.type",
		"`check`.offset",
		"`check`.frequency",
		"`check`.enabled",
		"`check`.state`",
		"`check`.state_change",
		"`check`.state_check",
		"`check`.settings",
		"`check`.health_settings",
		"`check`.created",
		"`check`.updated",
	)
	checks := make([]m.CheckForAlertDTO, 10)
	err := sess.Find(&checks)
	return checks, err
}

func ValidateCheckRoute(check *m.Check) error {
	sess, err := newSession(false, "check")
	if err != nil {
		return err
	}
	return validateCheckRoute(sess, check)
}

func validateCheckRoute(sess *session, check *m.Check) error {
	if !check.Enabled {
		return nil
	}
	switch check.Route.Type {
	case m.RouteByTags:
		if len(check.Route.Config["tags"].([]string)) == 0 {
			return m.NewValidationError("Need at least 1 tag defined in route config.")
		}
	case m.RouteByIds:
		sess.Table("probe")
		if len(check.Route.Config["ids"].([]int64)) == 0 {
			return m.NewValidationError("Need at least 1 valid id defined in route config.")
		}
		// get all checks.
		sess.Where("org_id=? OR public=1", check.OrgId).In("id", check.Route.Config["ids"].([]int64))
		results := make([]ProbeId, 0)
		err := sess.Find(&results)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			return m.NewValidationError("Need at least 1 valid id defined in route config.")
		}
		filteredIds := make([]int64, len(results))
		for i, row := range results {
			filteredIds[i] = row.Id
		}
		check.Route.Config["ids"] = filteredIds
	default:
		return m.NewValidationError(m.UnknownRouteType.Error())
	}
	return nil
}
