package infra_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/infra"
)

func TestNewHTTPClient_ClonedTunedTransport(t *testing.T) {
	t.Parallel()
	inj := do.New(infra.Package)
	c := do.MustInvoke[*infra.HTTPClient](inj)

	assert.NotNil(t, c.Client)
	assert.NotSame(t, http.DefaultTransport, c.Transport)
	tr, ok := c.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 100, tr.MaxIdleConns)
	assert.Equal(t, 20, tr.MaxIdleConnsPerHost)
}

func TestNewDBAndORM_FileSQLiteAndAutoMigrate(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "nested", "app.db")

	inj := do.New(infra.Package)
	do.ProvideValue(inj, &config.Config{
		Database: config.DatabaseConfig{
			Driver:      "sqlite",
			Connection:  dbPath + "?_pragma=foreign_keys(1)",
			AutoMigrate: true,
		},
	})
	t.Cleanup(func() { _ = inj.Shutdown() })

	orm := do.MustInvoke[*infra.ORM](inj)
	count, err := orm.Book.Query().Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestNewDB_InvalidDriverFailsFast(t *testing.T) {
	t.Parallel()
	inj := do.New(infra.Package)
	do.ProvideValue(inj, &config.Config{
		Database: config.DatabaseConfig{
			Driver:     "nonexistent_driver",
			Connection: "dsn",
		},
	})
	_, err := do.Invoke[*infra.DB](inj)
	require.Error(t, err)
}
