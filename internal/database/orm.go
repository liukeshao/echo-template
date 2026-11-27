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

// ORM 封装 *ent.Client 并实现 do.Shutdowner，由 DI 容器管理优雅关闭。
type ORM struct{ *ent.Client }

// Shutdown 关闭 ent 客户端及其底层连接驱动。
func (o *ORM) Shutdown() error { return o.Client.Close() }

// NewORM 基于 DI 中的 *DB 构造 ent 客户端，并按配置决定是否执行自动建表迁移。
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
