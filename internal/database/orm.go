package database

import (
	"context"
	"sync"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"

	_ "github.com/liukeshao/echo-template/ent/runtime"
)

// schemaCreateMu 串行化 Ent Schema.Create：Ent 生成的 ent/migrate.Tables 是包级全局变量，
// 建表期间会原地修改，串行化可确保并发测试（独立内存库）下零数据竞态。
var schemaCreateMu sync.Mutex

type ORM struct{ *ent.Client }

func (o *ORM) Shutdown() error { return o.Client.Close() }

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
			_ = client.Close()
			return nil, err
		}
	}
	return &ORM{client}, nil
}

func entDialect(driver string) string {
	if driver == "sqlite" {
		return dialect.SQLite
	}
	return driver
}
