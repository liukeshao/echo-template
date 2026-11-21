package infra

import (
	"context"
	"sync"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"

	_ "github.com/liukeshao/echo-template/ent/runtime"
	_ "modernc.org/sqlite"
)

// schemaCreateMu serializes Ent's Schema.Create calls because Ent's generated
// ent/migrate.Tables slice is package-global and mutated in place during schema
// setup. Serializing it has no runtime impact in production (runs once at
// startup) and keeps parallel tests with isolated in-memory databases race-free.
var schemaCreateMu sync.Mutex

// ORM wraps *ent.Client and implements do.Shutdowner so the DI
// container closes the client and its underlying connection during
// graceful shutdown.
type ORM struct{ *ent.Client }

// Shutdown closes the ent client (and its underlying *sql.DB driver).
func (o *ORM) Shutdown() error { return o.Client.Close() }

// NewORM builds an ent client from the *DB in the injector and runs
// automatic schema migration.
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
