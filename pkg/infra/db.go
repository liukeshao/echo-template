package infra

import (
	"database/sql"
	"os"
	"path/filepath"
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
	if isSQLiteDriver(driver) {
		if err := ensureSQLiteDir(connection); err != nil {
			return nil, err
		}
	}
	return sql.Open(driver, connection)
}

func isSQLiteDriver(driver string) bool {
	return driver == "sqlite" || driver == "sqlite3"
}

func ensureSQLiteDir(connection string) error {
	path := connection
	if i := strings.IndexRune(path, '?'); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimPrefix(path, "file:")
	if path == "" || path == ":memory:" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}
