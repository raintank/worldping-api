package sqlstore

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/raintank/worldping-api/pkg/events"
	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
)

type probeWithTag struct {
	m.Probe    `xorm:"extends"`
	m.ProbeTag `xorm:"extends"`
	RemoteIp   string
}

type probeWithTags []probeWithTag

func (probeWithTags) TableName() string {
	return "probe"
}

func (rows probeWithTags) ToProbeDTO() []m.ProbeDTO {
	probesById := make(map[int64]m.ProbeDTO)
	probeTagsById := make(map[int64]map[string]struct{})
	addressesById := make(map[int64]map[string]struct{})
	for _, r := range rows {
		_, ok := probesById[r.Probe.Id]
		if !ok {
			probesById[r.Probe.Id] = m.ProbeDTO{
				Id:            r.Probe.Id,
				Name:          r.Probe.Name,
				Slug:          r.Probe.Slug,
				Tags:          make([]string, 0),
				Enabled:       r.Probe.Enabled,
				EnabledChange: r.Probe.EnabledChange,
				OrgId:         r.Probe.OrgId,
				Public:        r.Probe.Public,
				Online:        r.Probe.Online,
				OnlineChange:  r.Probe.OnlineChange,
				Created:       r.Probe.Created,
				Updated:       r.Probe.Updated,
				Longitude:     r.Probe.Longitude,
				Latitude:      r.Probe.Latitude,
				RemoteIp:      make([]string, 0),
			}
			probeTagsById[r.Probe.Id] = make(map[string]struct{})
			if r.ProbeTag.Tag != "" {
				probeTagsById[r.Probe.Id][r.ProbeTag.Tag] = struct{}{}
			}
			addressesById[r.Probe.Id] = make(map[string]struct{})
			if r.RemoteIp != "" {
				addressesById[r.Probe.Id][r.RemoteIp] = struct{}{}
			}
		} else {
			if r.ProbeTag.Tag != "" {
				probeTagsById[r.Probe.Id][r.ProbeTag.Tag] = struct{}{}
			}
			if r.RemoteIp != "" {
				addressesById[r.Probe.Id][r.RemoteIp] = struct{}{}
			}
		}
	}
	probes := make([]m.ProbeDTO, len(probesById))
	i := 0
	for _, p := range probesById {
		for t := range probeTagsById[p.Id] {
			p.Tags = append(p.Tags, t)
		}
		for a := range addressesById[p.Id] {
			p.RemoteIp = append(p.RemoteIp, a)
		}
		probes[i] = p
		i++
	}
	return probes
}

func GetProbes(query *m.GetProbesQuery) ([]m.ProbeDTO, error) {
	sess, err := newSession(false, "probe")
	if err != nil {
		return nil, err
	}
	return getProbes(sess, query)
}

func getProbes(sess *session, query *m.GetProbesQuery) ([]m.ProbeDTO, error) {
	if query.OrgId == 0 {
		return nil, fmt.Errorf("GetProbesQuery requires OrgId to be set.")
	}
	var a probeWithTags
	var rawSQL bytes.Buffer
	args := make([]interface{}, 0)

	var where bytes.Buffer
	whereArgs := make([]interface{}, 0)
	prefix := "WHERE"

	fmt.Fprint(&rawSQL, "SELECT probe.*, probe_tag.*, probe_session.remote_ip FROM probe LEFT JOIN probe_tag ON  probe.id = probe_tag.probe_id AND probe_tag.org_id=? LEFT JOIN probe_session on probe_session.probe_id = probe.id ")
	args = append(args, query.OrgId)
	if query.Tag != "" {
		fmt.Fprint(&rawSQL, "INNER JOIN probe_tag as pt ON probe.id = pt.probe_id ")
		fmt.Fprintf(&where, "%s pt.tag = ? ", prefix)
		whereArgs = append(whereArgs, query.Tag)
		prefix = "AND"
	}

	if query.Name != "" {
		fmt.Fprintf(&where, "%s probe.name=? ", prefix)
		whereArgs = append(whereArgs, query.Name)
		prefix = "AND"
	}
	if query.Slug != "" {
		fmt.Fprintf(&where, "%s probe.slug=? ", prefix)
		whereArgs = append(whereArgs, query.Slug)
		prefix = "AND"
	}
	if query.Enabled != "" {
		enabled, err := strconv.ParseBool(query.Enabled)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&where, "%s probe.enabled=? ", prefix)
		whereArgs = append(whereArgs, enabled)
		prefix = "AND"
	}
	if query.Online != "" {
		online, err := strconv.ParseBool(query.Online)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&where, "%s probe.online=? ", prefix)
		whereArgs = append(whereArgs, online)
		prefix = "AND"
	}

	if query.Public != "" {
		public, err := strconv.ParseBool(query.Public)
		if err != nil {
			return nil, err
		}
		if public {
			fmt.Fprintf(&where, "%s probe.public=1 ", prefix)
			prefix = "AND"
		} else {
			fmt.Fprintf(&where, "%s (probe.public=0 AND probe.org_id=?) ", prefix)
			whereArgs = append(whereArgs, query.OrgId)
			prefix = "AND"
		}
	} else {
		fmt.Fprintf(&where, "%s (probe.org_id=? OR probe.public=1) ", prefix)
		whereArgs = append(whereArgs, query.OrgId)
		prefix = "AND"
	}

	if query.OrderBy == "" {
		query.OrderBy = "name"
	}

	fmt.Fprint(&rawSQL, where.String())
	args = append(args, whereArgs...)
	fmt.Fprintf(&rawSQL, "ORDER BY `%s` ASC", query.OrderBy)

	err := sess.Sql(rawSQL.String(), args...).Find(&a)

	if err != nil {
		return nil, err
	}
	return a.ToProbeDTO(), nil
}

func GetOnlineProbes() ([]m.Probe, error) {
	sess, err := newSession(false, "probe")
	if err != nil {
		return nil, err
	}
	return getOnlineProbes(sess)
}

func getOnlineProbes(sess *session) ([]m.Probe, error) {
	probes := make([]m.Probe, 0)
	err := sess.Where("probe.online=1").Find(&probes)
	if err != nil {
		return nil, err
	}
	return probes, nil
}

func onlineProbesWithNoSession(sess *session) ([]m.ProbeDTO, error) {
	sess.Table("probe")
	sess.Join("LEFT", "probe_tag", "probe.id=probe_tag.probe_id")
	sess.Join("LEFT", "probe_session", "probe.id = probe_session.probe_id")
	sess.Where("probe.online=1").And("probe_session.id is NULL")
	sess.Cols("`probe`.*", "`probe_tag`.*", "`probe_session`.remote_ip")
	var a probeWithTags
	err := sess.Find(&a)
	if err != nil {
		return nil, err
	}

	return a.ToProbeDTO(), nil
}

func GetProbeById(id int64, orgId int64) (*m.ProbeDTO, error) {
	sess, err := newSession(false, "probe")
	if err != nil {
		return nil, err
	}
	return getProbeById(sess, id, orgId)
}

func getProbeById(sess *session, id int64, orgId int64) (*m.ProbeDTO, error) {
	var a probeWithTags
	sess.Join("LEFT", "probe_tag", "probe.id = probe_tag.probe_id AND probe_tag.org_id=?", orgId)
	sess.Join("LEFT", "probe_session", "probe.id = probe_session.probe_id")
	sess.Where("probe.id=?", id)
	sess.And("probe.org_id=? OR probe.public=1", orgId)
	sess.Cols("`probe`.*", "`probe_tag`.*", "`probe_session`.remote_ip")
	err := sess.Find(&a)
	if err != nil {
		return nil, err
	}
	if len(a) == 0 {
		return nil, m.ErrProbeNotFound
	}
	return &a.ToProbeDTO()[0], nil
}

func GetProbeByName(name string, orgId int64) (*m.ProbeDTO, error) {
	sess, err := newSession(false, "probe")
	if err != nil {
		return nil, err
	}
	return getProbeByName(sess, name, orgId)
}

func getProbeByName(sess *session, name string, orgId int64) (*m.ProbeDTO, error) {
	var a probeWithTags
	sess.Where("probe.name=? AND probe.org_id=?", name, orgId)
	sess.Join("LEFT", "probe_tag", "probe.id = probe_tag.probe_id AND probe_tag.org_id=?", orgId)
	sess.Join("LEFT", "probe_session", "probe.id = probe_session.probe_id")
	sess.Cols("`probe`.*", "`probe_tag`.*", "`probe_session`.remote_ip")
	err := sess.Find(&a)
	if err != nil {
		return nil, err
	}
	if len(a) == 0 {
		return nil, m.ErrProbeNotFound
	}
	return &a.ToProbeDTO()[0], nil
}

func AddProbe(p *m.ProbeDTO) error {
	sess, err := newSession(true, "probe")
	if err != nil {
		return err
	}
	defer sess.Cleanup()
	if err = addProbe(sess, p); err != nil {
		return err
	}
	sess.Complete()
	return nil

}

func addProbe(sess *session, p *m.ProbeDTO) error {
	probe := &m.Probe{
		Name:          p.Name,
		Enabled:       p.Enabled,
		EnabledChange: time.Now(),
		OrgId:         p.OrgId,
		Public:        p.Public,
		Latitude:      p.Latitude,
		Longitude:     p.Longitude,
		Online:        false,
		OnlineChange:  time.Now(),
		Created:       time.Now(),
		Updated:       time.Now(),
	}
	probe.UpdateSlug()
	p.Slug = probe.Slug
	sess.UseBool("public")
	sess.UseBool("enabled")
	sess.UseBool("online")
	if _, err := sess.Insert(probe); err != nil {
		return err
	}
	p.Id = probe.Id
	p.Created = probe.Created
	p.Updated = probe.Updated

	p.OnlineChange = probe.OnlineChange
	p.EnabledChange = probe.EnabledChange

	probeTags := make([]m.ProbeTag, 0, len(p.Tags))
	for _, tag := range p.Tags {
		probeTags = append(probeTags, m.ProbeTag{
			OrgId:   p.OrgId,
			ProbeId: probe.Id,
			Tag:     tag,
			Created: time.Now(),
		})
	}
	if len(probeTags) > 0 {
		sess.Table("probe_tag")
		if _, err := sess.Insert(&probeTags); err != nil {
			return err
		}
	}
	events.Publish(&events.ProbeCreated{
		Ts:      p.Created,
		Payload: p,
	}, 0)
	return nil
}

func UpdateProbe(p *m.ProbeDTO) error {
	sess, err := newSession(true, "probe")
	if err != nil {
		return err
	}
	defer sess.Cleanup()

	err = updateProbe(sess, p)
	if err != nil {
		return err
	}
	sess.Complete()
	return err
}

func updateProbe(sess *session, p *m.ProbeDTO) error {
	existing, err := getProbeById(sess, p.Id, p.OrgId)
	if err != nil {
		return err
	}
	if existing == nil {
		return m.ErrProbeNotFound
	}
	if !existing.Public && p.OrgId != existing.OrgId {
		return m.ErrProbeNotFound
	}
	// If the OrgId is different, the only changes that can be made is to Tags.
	if p.OrgId == existing.OrgId {
		log.Debug("users owns probe, so can update all fields.")
		if existing.Enabled != p.Enabled {
			p.EnabledChange = time.Now()
		}
		probe := &m.Probe{
			Id:            p.Id,
			Name:          p.Name,
			Enabled:       p.Enabled,
			EnabledChange: p.EnabledChange,
			OrgId:         p.OrgId,
			Public:        p.Public,
			Created:       existing.Created,
			Updated:       time.Now(),
		}
		sess.UseBool("public")
		sess.UseBool("enabled")
		probe.UpdateSlug()
		p.Slug = probe.Slug
		if _, err := sess.Id(probe.Id).Update(probe); err != nil {
			return err
		}

		p.Updated = probe.Updated
	} else {
		log.Debug("user does not own probe, only tags can be updated.")
		tmp := *existing
		tmp.Tags = p.Tags
		tmp.OrgId = p.OrgId
		*p = tmp
	}

	tagMap := make(map[string]bool)
	tagsToDelete := make([]string, 0)
	tagsToAddMap := make(map[string]bool, 0)
	// create map of current tags
	for _, t := range existing.Tags {
		tagMap[t] = false
	}

	// create map of tags to add. We use a map
	// to ensure that we only add each tag once.
	for _, t := range p.Tags {
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
		rawParams := make([]interface{}, 0)
		rawParams = append(rawParams, p.Id, p.OrgId)
		q := make([]string, len(tagsToDelete))
		for i, t := range tagsToDelete {
			q[i] = "?"
			rawParams = append(rawParams, t)
		}
		rawSql := fmt.Sprintf("DELETE FROM probe_tag WHERE probe_id=? AND org_id=? AND tag IN (%s)", strings.Join(q, ","))
		if _, err := sess.Exec(rawSql, rawParams...); err != nil {
			return err
		}
	}
	if len(tagsToAdd) > 0 {
		newProbeTags := make([]m.ProbeTag, len(tagsToAdd))
		for i, tag := range tagsToAdd {
			newProbeTags[i] = m.ProbeTag{
				OrgId:   p.OrgId,
				ProbeId: p.Id,
				Tag:     tag,
				Created: time.Now(),
			}
		}
		sess.Table("probe_tag")
		if _, err := sess.Insert(&newProbeTags); err != nil {
			return err
		}
	}

	// dont emit events when only tags are changed.
	if p.OrgId == existing.OrgId {
		e := new(events.ProbeUpdated)
		e.Ts = p.Updated
		e.Payload.Current = p
		e.Payload.Last = existing
		events.Publish(e, 0)
	}

	return nil
}

type ProbeId struct {
	Id int64
}

func GetProbesForCheck(check *m.Check) ([]int64, error) {
	sess, err := newSession(false, "probe")
	if err != nil {
		return nil, err
	}

	return getProbesForCheck(sess, check)
}

func getProbesForCheck(sess *session, c *m.Check) ([]int64, error) {
	probes := make([]*ProbeId, 0)
	switch c.Route.Type {
	case m.RouteByTags:
		//TODO: this list needs to be filtered by probes that support the metrics listed in the task.
		tags := make([]string, len(c.Route.Config["tags"].([]string)))
		for i, tag := range c.Route.Config["tags"].([]string) {
			tags[i] = tag
		}
		sess.Join("LEFT", "probe_tag", "probe.id = probe_tag.probe_id AND probe_tag.org_id=?", c.OrgId)
		sess.In("probe_tag.tag", tags)
		sess.Distinct("probe.id")
		err := sess.Find(&probes)
		if err != nil {
			return nil, err
		}
	case m.RouteByIds:
		for _, id := range c.Route.Config["ids"].([]int64) {
			probes = append(probes, &ProbeId{Id: id})
		}
	default:
		return nil, fmt.Errorf("unknown routeType")
	}
	probeIds := make([]int64, len(probes))
	for i, a := range probes {
		probeIds[i] = a.Id
	}
	return probeIds, nil
}

func DeleteProbe(id int64, orgId int64) error {
	sess, err := newSession(true, "probe")
	if err != nil {
		return err
	}
	defer sess.Cleanup()
	err = deleteProbe(sess, id, orgId)
	if err != nil {
		return err
	}
	sess.Complete()
	return nil
}

func deleteProbe(sess *session, id int64, orgId int64) error {
	existing, err := getProbeById(sess, id, orgId)
	if err != nil {
		return err
	}
	if existing.OrgId != orgId {
		return m.ErrProbeNotFound
	}

	rawSql := "DELETE FROM probe WHERE id=? and org_id=?"
	if _, err := sess.Exec(rawSql, existing.Id, existing.OrgId); err != nil {
		return err
	}
	rawSql = "DELETE FROM probe_tag WHERE probe_id=? and org_id=?"
	if _, err := sess.Exec(rawSql, existing.Id, existing.OrgId); err != nil {
		return err
	}
	rawSql = "DELETE FROM route_by_id_index WHERE probe_id=?"
	if _, err := sess.Exec(rawSql, existing.Id); err != nil {
		return err
	}
	events.Publish(&events.ProbeDeleted{
		Ts:      time.Now(),
		Payload: existing,
	}, 0)
	return nil
}

func copyPublicProbeTags(sess *session, orgId int64) error {
	sess.Table("probe_tag")
	sess.Join("INNER", "probe", "probe.id=probe_tag.probe_id")
	sess.Where("probe.public=1").And("probe.org_id=probe_tag.org_id")
	result := make([]*m.ProbeTag, 0)
	err := sess.Find(&result)
	if err != nil {
		return err
	}

	if len(result) > 0 {
		probeTags := make([]m.ProbeTag, len(result))
		for i, probeTag := range result {
			probeTags[i] = m.ProbeTag{
				OrgId:   orgId,
				ProbeId: probeTag.ProbeId,
				Tag:     probeTag.Tag,
			}
		}
		sess.Table("probe_tag")
		if _, err := sess.Insert(&probeTags); err != nil {
			return err
		}
	}
	return nil

}

func AddProbeSession(probeSess *m.ProbeSession) error {
	sess, err := newSession(true, "probe_session")
	if err != nil {
		return err
	}
	defer sess.Cleanup()
	err = addProbeSession(sess, probeSess)
	if err != nil {
		return err
	}
	sess.Complete()
	return nil
}

func addProbeSession(sess *session, probeSess *m.ProbeSession) error {
	probeSess.Updated = time.Now()

	if _, err := sess.Insert(probeSess); err != nil {
		return err
	}
	rawSql := "UPDATE probe set online=1, online_change=? where id=?"
	if _, err := sess.Exec(rawSql, time.Now(), probeSess.ProbeId); err != nil {
		return err
	}
	log.Info("marking probeId=%d online as new session created.", probeSess.ProbeId)
	events.Publish(&events.ProbeSessionCreated{
		Ts:      probeSess.Updated,
		Payload: probeSess,
	}, 0)
	return nil

}

func GetProbeSessions(probeId int64, instance string) ([]m.ProbeSession, error) {
	sess, err := newSession(false, "probe_session")
	if err != nil {
		return nil, err
	}
	return getProbeSessions(sess, probeId, instance)
}

func getProbeSessions(sess *session, probeId int64, instance string) ([]m.ProbeSession, error) {
	if probeId != 0 {
		sess.And("probe_id=?", probeId)
	}
	if instance != "" {
		sess.And("instance_id=?", instance)
	}
	sessions := make([]m.ProbeSession, 0)
	err := sess.OrderBy("updated").Find(&sessions)
	return sessions, err

}

func DeleteProbeSession(p *m.ProbeSession) error {
	sess, err := newSession(true, "probe_session")
	if err != nil {
		return err
	}
	defer sess.Cleanup()
	err = deleteProbeSession(sess, p)
	if err != nil {
		return err
	}
	sess.Complete()
	return nil
}

func deleteProbeSession(sess *session, p *m.ProbeSession) error {
	existing := &m.ProbeSession{}
	has, err := sess.Where("org_id=? AND socket_id=?", p.OrgId, p.SocketId).Get(existing)
	if err != nil {
		return err
	}
	if !has {
		return nil
	}

	var rawSql = "DELETE FROM probe_session WHERE org_id=? AND socket_id=?"
	result, err := sess.Exec(rawSql, p.OrgId, p.SocketId)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if rowsAffected == 0 {
		//nothing was deleted. so no need to cleanup anything
		return nil
	}
	sessions, err := getProbeSessions(sess, existing.ProbeId, "")
	if err != nil {
		return err
	}
	if len(sessions) < 1 {
		log.Info("No sessions found for probeId=%d, marking probe as offline.", existing.ProbeId)
		rawSql := "UPDATE probe set online=0, online_change=? where id=?"
		if _, err := sess.Exec(rawSql, time.Now(), existing.ProbeId); err != nil {
			return err
		}
	}

	events.Publish(&events.ProbeSessionDeleted{
		Ts:      time.Now(),
		Payload: existing,
	}, 0)
	return nil
}

type probeOnlineSession struct {
	ProbeId   int64
	Online    bool
	SessionId int64
}

func ClearProbeSessions(instance string) error {
	sess, err := newSession(true, "probe_session")
	if err != nil {
		return err
	}
	defer sess.Cleanup()
	err = clearProbeSessions(sess, instance)
	if err != nil {
		return err
	}
	sess.Complete()
	return nil
}

func clearProbeSessions(sess *session, instance string) error {
	sessions, err := getProbeSessions(sess, 0, instance)
	if err != nil {
		return err
	}

	if len(sessions) > 0 {
		for _, s := range sessions {
			if err := deleteProbeSession(sess, &s); err != nil {
				return err
			}
		}
	}

	return nil
}

func GetProbeTags(orgId int64) ([]string, error) {
	sess, err := newSession(false, "probe_tag")
	if err != nil {
		return nil, err
	}
	return getProbeTags(sess, orgId)
}

func getProbeTags(sess *session, orgId int64) ([]string, error) {
	type tagRow struct {
		Tag string
	}
	rawSql := `SELECT DISTINCT(tag) as tag FROM probe_tag WHERE org_id=?`

	sess.Sql(rawSql, orgId)
	cTags := make([]tagRow, 0)
	if err := sess.Find(&cTags); err != nil {
		return nil, err
	}

	tags := make([]string, len(cTags))
	for i, tag := range cTags {
		tags[i] = tag.Tag
	}

	return tags, nil
}
