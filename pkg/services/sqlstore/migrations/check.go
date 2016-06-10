package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
	. "github.com/raintank/worldping-api/pkg/services/sqlstore/migrator"
)

func addCheckMigration(mg *Migrator) {

	// monitor v3
	var monitorV3 = Table{
		Name: "monitor",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "endpoint_id", Type: DB_BigInt, Nullable: false},
			{Name: "org_id", Type: DB_BigInt, Nullable: false},
			{Name: "namespace", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "monitor_type_id", Type: DB_BigInt, Nullable: false},
			{Name: "offset", Type: DB_BigInt, Nullable: false},
			{Name: "frequency", Type: DB_BigInt, Nullable: false},
			{Name: "enabled", Type: DB_Bool, Nullable: false},
			{Name: "settings", Type: DB_NVarchar, Length: 2048, Nullable: false},
			{Name: "state", Type: DB_BigInt, Nullable: false},
			{Name: "state_change", Type: DB_DateTime, Nullable: false},
			{Name: "created", Type: DB_DateTime, Nullable: false},
			{Name: "updated", Type: DB_DateTime, Nullable: false},
		}, Indices: []*Index{
			{Cols: []string{"monitor_type_id"}},
			{Cols: []string{"org_id", "namespace", "monitor_type_id"}, Type: UniqueIndex},
		},
	}

	// recreate table
	mg.AddMigration("create monitor v3", NewAddTableMigration(monitorV3))
	// recreate indices
	addTableIndicesMigrations(mg, "v3", monitorV3)

	//-------  drop indexes ------------------
	addDropAllIndicesMigrations(mg, "v3", monitorV3)

	//------- rename table ------------------
	addTableRenameMigration(mg, "monitor", "monitor_v3", "v3")

	var monitorV4 = Table{
		Name: "monitor",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "endpoint_id", Type: DB_BigInt, Nullable: false},
			{Name: "org_id", Type: DB_BigInt, Nullable: false},
			{Name: "monitor_type_id", Type: DB_BigInt, Nullable: false},
			{Name: "offset", Type: DB_BigInt, Nullable: false},
			{Name: "frequency", Type: DB_BigInt, Nullable: false},
			{Name: "enabled", Type: DB_Bool, Nullable: false},
			{Name: "settings", Type: DB_NVarchar, Length: 2048, Nullable: false},
			{Name: "state", Type: DB_BigInt, Nullable: false},
			{Name: "state_change", Type: DB_DateTime, Nullable: false},
			{Name: "created", Type: DB_DateTime, Nullable: false},
			{Name: "updated", Type: DB_DateTime, Nullable: false},
		}, Indices: []*Index{
			{Cols: []string{"monitor_type_id"}},
			{Cols: []string{"org_id", "endpoint_id", "monitor_type_id"}, Type: UniqueIndex},
		},
	}

	// recreate table
	mg.AddMigration("create monitor v4", NewAddTableMigration(monitorV4))
	// recreate indices
	addTableIndicesMigrations(mg, "v4", monitorV4)
	//------- copy data from v1 to v2 -------------------
	mg.AddMigration("copy monitor v3 to v4", NewCopyTableDataMigration("monitor", "monitor_v3", map[string]string{
		"id":              "id",
		"endpoint_id":     "endpoint_id",
		"org_id":          "org_id",
		"monitor_type_id": "monitor_type_id",
		"offset":          "offset",
		"frequency":       "frequency",
		"enabled":         "enabled",
		"settings":        "settings",
		"state":           "state",
		"state_change":    "state_change",
		"created":         "created",
		"updated":         "updated",
	}))
	mg.AddMigration("Drop old table monitor_v3", NewDropTableMigration("monitor_v3"))

	//monitorTypes
	var monitorTypeV1 = Table{
		Name: "monitor_type",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "name", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "created", Type: DB_DateTime, Nullable: false},
			{Name: "updated", Type: DB_DateTime, Nullable: false},
		},
	}
	mg.AddMigration("create monitor_type table", NewAddTableMigration(monitorTypeV1))

	//monitorTypesSettings
	var monitorTypeSettingV1 = Table{
		Name: "monitor_type_setting",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "monitor_type_id", Type: DB_BigInt, Nullable: false},
			{Name: "variable", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "description", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "data_type", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "conditions", Type: DB_NVarchar, Length: 1024, Nullable: false},
			{Name: "default_value", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "required", Type: DB_Bool, Nullable: false},
			{Name: "created", Type: DB_DateTime, Nullable: false},
			{Name: "updated", Type: DB_DateTime, Nullable: false},
		},
		Indices: []*Index{
			{Cols: []string{"monitor_type_id"}},
		},
	}
	mg.AddMigration("create monitor_type_setting table", NewAddTableMigration(monitorTypeSettingV1))

	//-------  indexes ------------------
	mg.AddMigration("add index monitor_type_setting.monitor_type_id", NewAddIndexMigration(monitorTypeSettingV1, monitorTypeSettingV1.Indices[0]))

	//monitorCollector
	var monitorCollectorV1 = Table{
		Name: "monitor_collector",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "monitor_id", Type: DB_BigInt, Nullable: false},
			{Name: "collector_id", Type: DB_BigInt, Nullable: false},
		},
		Indices: []*Index{
			{Cols: []string{"monitor_id", "collector_id"}},
		},
	}
	mg.AddMigration("create monitor_collector table", NewAddTableMigration(monitorCollectorV1))

	//-------  indexes ------------------
	addTableIndicesMigrations(mg, "v1", monitorCollectorV1)

	// add monitor_collector_tags
	var monitorCollectorTagV1 = Table{
		Name: "monitor_collector_tag",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "monitor_id", Type: DB_BigInt, Nullable: false},
			{Name: "tag", Type: DB_NVarchar, Length: 255, Nullable: false},
		},
		Indices: []*Index{
			{Cols: []string{"monitor_id"}},
			{Cols: []string{"monitor_id", "tag"}, Type: UniqueIndex},
		},
	}
	mg.AddMigration("create monitor_collector_tag table v1", NewAddTableMigration(monitorCollectorTagV1))

	//-------  indexes ------------------
	addTableIndicesMigrations(mg, "v1", monitorCollectorTagV1)

	// add state_check field
	migration := NewAddColumnMigration(monitorV4, &Column{
		Name: "state_check", Type: DB_DateTime, Nullable: true,
	})
	mg.AddMigration("monitor add state_check v1", migration)

	// add health settings
	migration = NewAddColumnMigration(monitorV4, &Column{
		Name: "health_settings", Type: DB_NVarchar, Length: 2048, Nullable: true, Default: "",
	})
	migration.OnSuccess = func(sess *xorm.Session) error {
		sess.Table("monitor")
		monitors := make([]m.Monitor, 0)
		if err := sess.Find(&monitors); err != nil {
			return err
		}
		for _, mon := range monitors {

			if (mon.HealthSettings != nil) && (mon.HealthSettings.Steps != 0) && (mon.HealthSettings.NumProbes != 0) {
				continue
			}
			if mon.HealthSettings == nil {
				mon.HealthSettings = &m.CheckHealthSettings{NumProbes: 1, Steps: 2}
			} else {
				mon.HealthSettings.NumProbes = 1
				mon.HealthSettings.Steps = 2
			}
			if _, err := sess.Id(mon.Id).Update(mon); err != nil {
				return err
			}
		}
		return nil
	}
	mg.AddMigration("monitor add alerts v1", migration)

	// add health settings
	migration = NewAddColumnMigration(monitorV4, &Column{
		Name: "type", Type: DB_NVarchar, Length: 32, Nullable: true, Default: "",
	})
	migration.OnSuccess = func(sess *xorm.Session) error {
		sess.Table("monitor")
		q := "UPDATE monitor set type=? where monitor_type_id=?"
		for t, id := range map[string]int{"http": 1, "https": 2, "ping": 3, "dns": 4} {
			_, err := sess.Exec(q, t, id)
			if err != nil {
				return err
			}
		}
		return nil
	}
	mg.AddMigration("monitor add type v1", migration)

	// rename monitor to check
	//-------  drop indexes ------------------
	addDropAllIndicesMigrations(mg, "v4", monitorV4)

	var checkV1 = Table{
		Name: "check",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "endpoint_id", Type: DB_BigInt, Nullable: false},
			{Name: "org_id", Type: DB_BigInt, Nullable: false},
			{Name: "type", Type: DB_NVarchar, Length: 16, Nullable: false},
			{Name: "route", Type: DB_NVarchar, Length: 2048, Nullable: true},
			{Name: "offset", Type: DB_BigInt, Nullable: false},
			{Name: "frequency", Type: DB_BigInt, Nullable: false},
			{Name: "enabled", Type: DB_Bool, Nullable: false},
			{Name: "settings", Type: DB_Text, Nullable: false},
			{Name: "health_settings", Type: DB_Text, Nullable: false},
			{Name: "state", Type: DB_BigInt, Nullable: false},
			{Name: "state_change", Type: DB_DateTime, Nullable: false},
			{Name: "state_check", Type: DB_DateTime, Nullable: false},
			{Name: "created", Type: DB_DateTime, Nullable: false},
			{Name: "updated", Type: DB_DateTime, Nullable: false},
		}, Indices: []*Index{
			{Cols: []string{"org_id", "endpoint_id", "type"}, Type: UniqueIndex},
		},
	}

	// recreate table
	mg.AddMigration("create check v1", NewAddTableMigration(checkV1))
	// recreate indices
	addTableIndicesMigrations(mg, "v1", checkV1)

	//------- copy data from v1 to v2 -------------------
	checkTableMigration := NewCopyTableDataMigration("check", "monitor", map[string]string{
		"id":              "id",
		"endpoint_id":     "endpoint_id",
		"org_id":          "org_id",
		"type":            "type",
		"offset":          "offset",
		"frequency":       "frequency",
		"enabled":         "enabled",
		"settings":        "settings",
		"health_settings": "health_settings",
		"state":           "state",
		"state_check":     "state_check",
		"state_change":    "state_change",
		"created":         "created",
		"updated":         "updated",
	})

	checkTableMigration.OnSuccess = func(sess *xorm.Session) error {
		type byIdRow struct {
			MonitorId   int64
			CollectorId int64
		}
		byIds := make([]byIdRow, 0)
		err := sess.Table("monitor_collector").Cols("monitor_id", "collector_id").Find(&byIds)
		if err != nil {
			return err
		}
		type rById struct {
			MonitorId    int64
			CollectorIds []int64
		}
		routes := make(map[int64]*rById)

		for _, row := range byIds {
			r, ok := routes[row.MonitorId]
			if !ok {
				r = &rById{MonitorId: row.MonitorId, CollectorIds: make([]int64, 0)}
				routes[row.MonitorId] = r
			}
			r.CollectorIds = append(r.CollectorIds, row.CollectorId)
		}

		for _, r := range routes {
			check := m.Check{Id: r.MonitorId, Route: &m.CheckRoute{
				Type: m.RouteByIds,
				Config: map[string]interface{}{
					"ids": r.CollectorIds,
				},
			}}
			_, err := sess.Cols("route").Id(check.Id).Update(&check)
			if err != nil {
				return err
			}
		}

		type byTagRow struct {
			MonitorId int64
			Tag       string
		}
		byTags := make([]byTagRow, 0)
		err = sess.Table("monitor_collector_tag").Cols("monitor_id", "tag").Find(&byTags)
		if err != nil {
			return err
		}

		type rByTag struct {
			MonitorId int64
			Tags      []string
		}
		routesByTag := make(map[int64]*rByTag)
		for _, row := range byTags {
			if _, ok := routes[row.MonitorId]; ok {
				log.Info("ERROR: check %d has both collector_tags and collector_ids", row.MonitorId)
				continue
			}
			r, ok := routesByTag[row.MonitorId]
			if !ok {
				r = &rByTag{MonitorId: row.MonitorId, Tags: make([]string, 0)}
				routesByTag[row.MonitorId] = r
			}
			r.Tags = append(r.Tags, row.Tag)
		}
		for _, r := range routesByTag {
			fmt.Printf("check %d has %d tags\n", r.MonitorId, len(r.Tags))
			check := m.Check{Id: r.MonitorId, Route: &m.CheckRoute{
				Type: m.RouteByTags,
				Config: map[string]interface{}{
					"tags": r.Tags,
				},
			}}
			_, err := sess.Cols("route").Id(check.Id).Update(&check)
			if err != nil {
				return err
			}
		}

		//update check settings
		type tmpCheck struct {
			Settings []m.MonitorSettingDTO
			Id       int64
			Type     m.CheckType
		}
		checks := make([]tmpCheck, 0)
		err = sess.Table("check").Find(&checks)
		if err != nil {
			return err
		}

		for _, c := range checks {
			newCheck := m.Check{Id: c.Id}
			newCheck.Settings = m.MonitorSettingsDTO(c.Settings).ToV2Setting(c.Type)
			_, err := sess.Id(c.Id).Cols("settings").Update(&newCheck)
			if err != nil {
				return err
			}

			// set route on any checks that have a null route still.
			if _, ok := routes[c.Id]; !ok {
				if _, ok := routesByTag[c.Id]; !ok {
					checkWithoutRoute := m.Check{Id: c.Id, Route: &m.CheckRoute{Type: m.RouteByIds, Config: map[string]interface{}{"ids": []int64{}}}}
					log.Info("ERROR: Check %d has no probes set.", c.Id)
					_, err := sess.Id(c.Id).Cols("route").Update(&checkWithoutRoute)
					if err != nil {
						return err
					}
				}
			}
		}

		return err
	}

	mg.AddMigration("copy monitor v4 to check v1", checkTableMigration)
	mg.AddMigration("Drop old table monitor_v4", NewDropTableMigration("monitor"))

	// remove monitor_type and monitor_type_setting as data is just hard coded in the APP.
	mg.AddMigration("Drop monitor_type", NewDropTableMigration("monitor_type"))
	mg.AddMigration("Drop monitor_type_setting", NewDropTableMigration("monitor_type_setting"))

	//rename monitor_collector to RouteByIdIndex
	routeIndexV1 := Table{
		Name: "route_by_id_index",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "check_id", Type: DB_BigInt, Nullable: false},
			{Name: "probe_id", Type: DB_BigInt, Nullable: false},
			{Name: "created", Type: DB_DateTime, Nullable: true},
		},
		Indices: []*Index{
			{Cols: []string{"check_id", "probe_id"}, Type: UniqueIndex},
		},
	}
	mg.AddMigration("create route_by_id_index table v1", NewAddTableMigration(routeIndexV1))
	for _, index := range routeIndexV1.Indices {
		migrationId := fmt.Sprintf("create index %s - %s", index.XName(routeIndexV1.Name), "v1")
		mg.AddMigration(migrationId, NewAddIndexMigration(routeIndexV1, index))
	}

	//------- copy data from v1 to v2 -------------------
	mg.AddMigration("copy monitor_collector to route_by_id_index v1", NewCopyTableDataMigration("route_by_id_index", "monitor_collector", map[string]string{
		"id":       "id",
		"check_id": "monitor_id",
		"probe_id": "collector_id",
	}))
	mg.AddMigration("Drop old table monitor_collector", NewDropTableMigration("monitor_collector"))

	//rename monitor_collector to RouteByIdIndex
	routeTagIndexV1 := Table{
		Name: "route_by_tag_index",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "org_id", Type: DB_BigInt, Nullable: false},
			{Name: "check_id", Type: DB_BigInt, Nullable: false},
			{Name: "tag", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "created", Type: DB_DateTime},
		},
		Indices: []*Index{
			{Cols: []string{"check_id", "tag"}, Type: UniqueIndex},
			{Cols: []string{"org_id", "tag"}},
		},
	}
	mig := NewAddTableMigration(routeTagIndexV1)
	mig.OnSuccess = func(sess *xorm.Session) error {
		rawSQL := "INSERT INTO route_by_tag_index SELECT ct.id, c.org_id, ct.monitor_id, ct.tag, c.updated from monitor_collector_tag as ct JOIN `check` as c on ct.monitor_id=c.id"
		sess.Table("route_by_tag_index")
		_, err := sess.Exec(rawSQL)
		return err
	}
	mg.AddMigration("create route_by_tag_index table v1", mig)

	for _, index := range routeTagIndexV1.Indices {
		migrationId := fmt.Sprintf("create index %s - %s", index.XName(routeTagIndexV1.Name), "v1")
		mg.AddMigration(migrationId, NewAddIndexMigration(routeTagIndexV1, index))
	}

	mg.AddMigration("Drop old table monitor_collector_tag", NewDropTableMigration("monitor_collector_tag"))
}
