package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"

	_ "modernc.org/sqlite"
)

type DB struct{ *sql.DB }

func (d *DB) Shutdown() error { return d.DB.Close() }

func NewDB(i do.Injector) (*DB, error) {
	cfg := do.MustInvoke[*config.Config](i)
	raw, err := openDB(cfg.Database.Driver, cfg.Database.Connection)
	if err != nil {
		return nil, err
	}

	// SQLite 仅支持单写并发，限制最大打开连接数为 1 以避免高并发下的 SQLITE_BUSY 锁冲突。
	if isSQLiteDriver(cfg.Database.Driver) {
		raw.SetMaxOpenConns(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := raw.PingContext(ctx); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("database: ping database: %w", err)
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
