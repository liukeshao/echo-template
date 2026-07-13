package infra

import (
	"context"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"

	_ "github.com/liukeshao/echo-template/ent/runtime"
	_ "modernc.org/sqlite"
)

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

	if err := client.Schema.Create(context.Background()); err != nil {
		return nil, err
	}
	return &ORM{client}, nil
}

func entDialect(driver string) string {
	if driver == "sqlite" {
		return dialect.SQLite
	}
	return driver
}
