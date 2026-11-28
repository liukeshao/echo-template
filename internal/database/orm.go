package database

import (
	"context"
	"sync"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/samber/do/v2"

	"github.com/liukeshao/go-rest-template/config"
	"github.com/liukeshao/go-rest-template/ent"

	_ "github.com/liukeshao/go-rest-template/ent/runtime"
)

// schemaCreateMu 串行化 Ent Schema.Create：Ent 生成的 ent/migrate.Tables 是包级全局变量，
// 建表期间会原地修改，串行化可确保并发测试（独立内存库）下零数据竞态。
var schemaCreateMu sync.Mutex

// ORM 包装 *ent.Client；底层 *sql.DB 连接生命周期由 *DB 单一管理。
type ORM struct{ *ent.Client }

func NewORM(i do.Injector) (*ORM, error) {
	db := do.MustInvoke[*DB](i)
	cfg := do.MustInvoke[*config.Config](i)

	drv := entsql.OpenDB(entDialect(cfg.Database.Driver), db.DB)
	client := ent.NewClient(ent.Driver(drv))

	if cfg.Database.AutoMigrate {
		schemaCreateMu.Lock()
		err := client.Schema.Create(context.Background())
		schemaCreateMu.Unlock()
		if err != nil {
			return nil, err
		}
	}
	return &ORM{client}, nil
}

func entDialect(driver string) string {
	switch driver {
	case "sqlite", "sqlite3":
		return dialect.SQLite
	case "pgx", "postgres", "postgresql":
		return dialect.Postgres
	case "mysql":
		return dialect.MySQL
	default:
		return driver
	}
}
