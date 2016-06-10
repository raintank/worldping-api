package migrations

import . "github.com/raintank/worldping-api/pkg/services/sqlstore/migrator"

// --- Migration Guide line ---
// 1. Never change a migration that is committed and pushed to master
// 2. Always add new migrations (to change or undo previous migrations)
// 3. Some migraitons are not yet written (rename column, table, drop table, index etc)

func AddMigrations(mg *Migrator) {
	addMigrationLogMigrations(mg)
	addCollectorMigration(mg)
	addCheckMigration(mg)
	addEndpointMigration(mg)
	addAlertSchedulerValueMigration(mg)
	addQuotaMigration(mg)
}

func addMigrationLogMigrations(mg *Migrator) {
	migrationLogV1 := Table{
		Name: "migration_log",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "migration_id", Type: DB_NVarchar, Length: 255},
			{Name: "sql", Type: DB_Text},
			{Name: "success", Type: DB_Bool},
			{Name: "error", Type: DB_Text},
			{Name: "timestamp", Type: DB_DateTime},
		},
	}

	mg.AddMigration("create migration_log table", NewAddTableMigration(migrationLogV1))
}
