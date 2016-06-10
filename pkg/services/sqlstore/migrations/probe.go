package migrations

import (
	"fmt"
	"time"

	"github.com/go-xorm/xorm"
	. "github.com/raintank/worldping-api/pkg/services/sqlstore/migrator"
)

func addCollectorMigration(mg *Migrator) {

	var collectorV1 = Table{
		Name: "collector",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "org_id", Type: DB_BigInt, Nullable: false},
			{Name: "slug", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "name", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "latitude", Type: DB_Float, Nullable: true},
			{Name: "longitude", Type: DB_Float, Nullable: true},
			{Name: "public", Type: DB_Bool, Nullable: false},
			{Name: "online", Type: DB_Bool, Nullable: false},
			{Name: "enabled", Type: DB_Bool, Nullable: false},
			{Name: "created", Type: DB_DateTime, Nullable: false},
			{Name: "updated", Type: DB_DateTime, Nullable: false},
		},
		Indices: []*Index{
			{Cols: []string{"org_id", "slug"}, Type: UniqueIndex},
		},
	}
	mg.AddMigration("create collector table v1", NewAddTableMigration(collectorV1))

	//-------  indexes ------------------
	addTableIndicesMigrations(mg, "v1", collectorV1)

	// add location_tag
	var collectorTagV1 = Table{
		Name: "collector_tag",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "org_id", Type: DB_BigInt, Nullable: false},
			{Name: "collector_id", Type: DB_BigInt, Nullable: false},
			{Name: "tag", Type: DB_NVarchar, Length: 255, Nullable: false},
		},
		Indices: []*Index{
			{Cols: []string{"org_id", "collector_id"}},
			{Cols: []string{"collector_id", "org_id", "tag"}, Type: UniqueIndex},
		},
	}
	mg.AddMigration("create collector_tag table v1", NewAddTableMigration(collectorTagV1))

	//-------  indexes ------------------
	addTableIndicesMigrations(mg, "v1", collectorTagV1)

	//CollectorSession
	var collectorSessionV1 = Table{
		Name: "collector_session",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "org_id", Type: DB_BigInt, Nullable: false},
			{Name: "collector_id", Type: DB_BigInt, Nullable: false},
			{Name: "socket_id", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "updated", Type: DB_DateTime, Nullable: false},
		},
		Indices: []*Index{
			{Cols: []string{"org_id"}},
			{Cols: []string{"collector_id"}},
			{Cols: []string{"socket_id"}},
		},
	}
	mg.AddMigration("create collector_session table", NewAddTableMigration(collectorSessionV1))
	//-------  indexes ------------------
	instanceCol := &Column{Name: "instance_id", Type: DB_NVarchar, Length: 255, Nullable: true}
	migration := NewAddColumnMigration(collectorSessionV1, instanceCol)
	migration.OnSuccess = func(sess *xorm.Session) error {
		rawSql := "DELETE FROM collector_session"
		sess.Table("collector_session")
		_, err := sess.Exec(rawSql)
		return err
	}
	mg.AddMigration("add instance_id to collector_session table v1", migration)

	//add onlineChange, enabledChange columns
	mg.AddMigration("add online_change col to collector table v1",
		NewAddColumnMigration(collectorV1,
			&Column{Name: "online_change", Type: DB_DateTime, Nullable: true}))

	changeCol := &Column{Name: "enabled_change", Type: DB_DateTime, Nullable: true}
	addEnableChangeMig := NewAddColumnMigration(collectorV1, changeCol)
	addEnableChangeMig.OnSuccess = func(sess *xorm.Session) error {
		rawSQL := "UPDATE collector set enabled_change=?"
		sess.Table("collector")
		_, err := sess.Exec(rawSQL, time.Now())
		return err
	}
	mg.AddMigration("add enabled_change col to collector table v1", addEnableChangeMig)

	// rename collector to probe
	addTableRenameMigration(mg, "collector", "probe", "v1")

	// rename collector_tag  to probe_tag
	probeTagV1 := Table{
		Name: "probe_tag",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "org_id", Type: DB_BigInt, Nullable: false},
			{Name: "probe_id", Type: DB_BigInt, Nullable: false},
			{Name: "tag", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "created", Type: DB_DateTime, Nullable: true},
		},
		Indices: []*Index{
			{Cols: []string{"org_id", "probe_id"}},
			{Cols: []string{"org_id", "tag", "probe_id"}, Type: UniqueIndex},
		},
	}
	mg.AddMigration("create probe_tag table v1", NewAddTableMigration(probeTagV1))
	for _, index := range probeTagV1.Indices {
		migrationId := fmt.Sprintf("create index %s - %s", index.XName(probeTagV1.Name), "v1")
		mg.AddMigration(migrationId, NewAddIndexMigration(probeTagV1, index))
	}
	mg.AddMigration("copy collector_tag to probe_tag v1", NewCopyTableDataMigration("probe_tag", "collector_tag", map[string]string{
		"id":       "id",
		"org_id":   "org_id",
		"probe_id": "collector_id",
		"tag":      "tag",
	}))
	mg.AddMigration("Drop old table collector_tag", NewDropTableMigration("collector_tag"))

	// rename collector_session to probe_session
	probeSessionV1 := Table{
		Name: "probe_session",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "org_id", Type: DB_BigInt, Nullable: false},
			{Name: "probe_id", Type: DB_BigInt, Nullable: false},
			{Name: "socket_id", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "instance_id", Type: DB_NVarchar, Length: 255, Nullable: false},
			{Name: "version", Type: DB_NVarchar, Length: 16, Nullable: false},
			{Name: "remote_ip", Type: DB_NVarchar, Length: 48, Nullable: false},
			{Name: "updated", Type: DB_DateTime, Nullable: false},
		},
		Indices: []*Index{
			{Cols: []string{"org_id"}},
			{Cols: []string{"probe_id"}},
			{Cols: []string{"socket_id"}},
			{Cols: []string{"instance_id"}},
		},
	}
	mg.AddMigration("create probe_session table", NewAddTableMigration(probeSessionV1))
	for _, index := range probeSessionV1.Indices {
		migrationId := fmt.Sprintf("create index %s - %s", index.XName(probeSessionV1.Name), "v1")
		mg.AddMigration(migrationId, NewAddIndexMigration(probeSessionV1, index))
	}
	mg.AddMigration("Drop old table collector_session", NewDropTableMigration("collector_session"))

}
