package database_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/go-rest-template/config"
	"github.com/liukeshao/go-rest-template/internal/database"
)

func TestNewDBAndORM_FileSQLiteAndAutoMigrate(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "nested", "app.db")

	inj := do.New(database.Package)
	do.ProvideValue(inj, &config.Config{
		Database: config.Database{
			Driver:      "sqlite",
			Connection:  dbPath + "?_pragma=foreign_keys(1)",
			AutoMigrate: true,
		},
	})
	t.Cleanup(func() { _ = inj.Shutdown() })

	orm := do.MustInvoke[*database.ORM](inj)
	count, err := orm.Book.Query().Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestNewDB_InvalidDriverFailsFast(t *testing.T) {
	t.Parallel()
	inj := do.New(database.Package)
	do.ProvideValue(inj, &config.Config{
		Database: config.Database{
			Driver:     "nonexistent_driver",
			Connection: "dsn",
		},
	})
	_, err := do.Invoke[*database.DB](inj)
	require.Error(t, err)
}
