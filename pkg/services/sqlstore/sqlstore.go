package sqlstore

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/services/sqlstore/migrations"
	"github.com/raintank/worldping-api/pkg/services/sqlstore/migrator"
	"github.com/raintank/worldping-api/pkg/services/sqlstore/sqlutil"
	"github.com/raintank/worldping-api/pkg/setting"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	_ "github.com/mattn/go-sqlite3"
)

var (
	x       *xorm.Engine
	dialect migrator.Dialect
	l       sync.Mutex
	DbCfg   struct {
		Type, Host, Name, User, Pwd, Path, SslMode string
	}

	UseSQLite3 bool
)

// setup and manage an SQLite3 engine for testing.
func MockEngine() error {
	l.Lock()
	defer l.Unlock()
	if x != nil {
		log.Info("cleaning existing DB")
		sqlutil.CleanDB(x)
		migrator := migrator.NewMigrator(x)
		migrator.LogLevel = log.INFO
		migrations.AddMigrations(migrator)

		return migrator.Start()
	}
	e, err := xorm.NewEngine(sqlutil.TestDB_Sqlite3.DriverName, sqlutil.TestDB_Sqlite3.ConnStr)
	//x, err := xorm.NewEngine(sqlutil.TestDB_Mysql.DriverName, sqlutil.TestDB_Mysql.ConnStr)
	//x, err := xorm.NewEngine(sqlutil.TestDB_Postgres.DriverName, sqlutil.TestDB_Postgres.ConnStr)
	e.SetMaxOpenConns(1)
	if err != nil {
		return err
	}

	sqlutil.CleanDB(e)

	return SetEngine(e, false)
}

func NewEngine() {
	x, err := getEngine()

	if err != nil {
		log.Fatal(3, "Sqlstore: Fail to connect to database: %v", err)
	}

	err = SetEngine(x, setting.Env == setting.DEV)

	if err != nil {
		log.Fatal(3, "fail to initialize orm engine: %v", err)
	}
	x.SetMaxOpenConns(20)
}

func SetEngine(engine *xorm.Engine, enableLog bool) (err error) {
	x = engine

	dialect = migrator.NewDialect(x.DriverName())

	migrator := migrator.NewMigrator(x)
	migrator.LogLevel = log.INFO
	migrations.AddMigrations(migrator)

	if err := migrator.Start(); err != nil {
		return fmt.Errorf("Sqlstore::Migration failed err: %v\n", err)
	}

	if enableLog {
		logPath := path.Join(setting.LogsPath, "xorm.log")
		os.MkdirAll(path.Dir(logPath), os.ModePerm)

		f, err := os.Create(logPath)
		if err != nil {
			return fmt.Errorf("sqlstore.init(fail to create xorm.log): %v", err)
		}
		x.SetLogger(xorm.NewSimpleLogger(f))
		if setting.Env == setting.DEV {
			x.ShowSQL(true)
		}
	}
	return nil
}

func getEngine() (*xorm.Engine, error) {
	LoadConfig()

	cnnstr := ""
	switch DbCfg.Type {
	case "mysql":
		cnnstr = fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8",
			DbCfg.User, DbCfg.Pwd, DbCfg.Host, DbCfg.Name)
	case "sqlite3":
		if !filepath.IsAbs(DbCfg.Path) {
			DbCfg.Path = filepath.Join(setting.DataPath, DbCfg.Path)
		}
		os.MkdirAll(path.Dir(DbCfg.Path), os.ModePerm)
		cnnstr = "file:" + DbCfg.Path + "?cache=shared&mode=rwc&_loc=Local"
	default:
		return nil, fmt.Errorf("Unknown database type: %s", DbCfg.Type)
	}

	log.Info("Database: %v", DbCfg.Type)

	return xorm.NewEngine(DbCfg.Type, cnnstr)
}

func LoadConfig() {
	sec := setting.Cfg.Section("database")

	DbCfg.Type = sec.Key("type").String()
	if DbCfg.Type == "sqlite3" {
		UseSQLite3 = true
	}
	DbCfg.Host = sec.Key("host").String()
	DbCfg.Name = sec.Key("name").String()
	DbCfg.User = sec.Key("user").String()
	if len(DbCfg.Pwd) == 0 {
		DbCfg.Pwd = sec.Key("password").String()
	}
	DbCfg.SslMode = sec.Key("ssl_mode").String()
	DbCfg.Path = sec.Key("path").MustString("data/grafana.db")
}

func TestDB() error {
	sess, err := newSession(true, "endpoint")
	if err != nil {
		return err
	}
	defer sess.Cleanup()

	if err = testDB(sess); err != nil {
		return err
	}
	sess.Complete()
	return nil
}

func testDB(sess *session) error {
	_, err := sess.Query("SELECT 1")
	return err
}
