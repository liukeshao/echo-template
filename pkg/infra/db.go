package infra

import (
	"database/sql"
	"os"
	"strings"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
)

// DB wraps *sql.DB and implements do.Shutdowner so the DI container
// can close the connection during graceful shutdown.
type DB struct{ *sql.DB }

// Shutdown closes the underlying database connection pool.
func (d *DB) Shutdown() error { return d.DB.Close() }

// NewDB opens the SQL connection pool using config from the injector.
func NewDB(i do.Injector) (*DB, error) {
	cfg := do.MustInvoke[*config.Config](i)
	raw, err := openDB(cfg.Database.Driver, cfg.Database.Connection)
	if err != nil {
		return nil, err
	}
	return &DB{raw}, nil
}

func openDB(driver, connection string) (*sql.DB, error) {
	if driver == "sqlite3" {
		parts := strings.Split(connection, "/")
		if len(parts) > 1 {
			dir := strings.Join(parts[:len(parts)-1], "/")
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, err
			}
		}
	}
	return sql.Open(driver, connection)
}
