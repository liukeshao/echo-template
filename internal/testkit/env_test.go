package testkit_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/internal/testkit"
)

func TestNew_WithConfigFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	cfgPath := filepath.Join(dir, "config.test.toml")

	tomlContent := `
[http]
hostname = "127.0.0.1"
port = 9090
graceful_timeout = "2s"
read_timeout = "5s"
write_timeout = "10s"
idle_timeout = "1m"

[app]
name = "from-file"
host = "http://localhost:9090"
environment = "test"

[auth]
bcrypt_cost = 4

[database]
driver = "sqlite"
connection = "` + filepath.ToSlash(dbPath) + `?_pragma=foreign_keys(1)"
auto_migrate = true

[jwt]
secret = "file-secret-key-at-least-32-bytes-long"
access_token_expiry = "30m"
refresh_token_expiry = "48h"
issuer = "file-issuer"

[clients.weather]
base_url = "https://weather.test.local/v1"
api_key = "file-api-key"
timeout = "3s"
retry = 1

[i18n]
default_locale = "zh-CN"
supported_locales = ["en", "zh-CN"]
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(tomlContent), 0644))
	t.Setenv("ECHO_TEMPLATE_CLIENTS_WEATHER_API_KEY", "env-api-key")

	env := testkit.New(t, testkit.WithConfigFile(cfgPath))

	assert.Equal(t, "from-file", env.Config.App.Name)
	assert.Equal(t, uint16(9090), env.Config.HTTP.Port)
	assert.Equal(t, 30*time.Minute, env.Config.JWT.AccessTokenExpiry)
	assert.Equal(t, "env-api-key", env.Config.Clients.Weather.APIKey)
	assert.Equal(t, "zh-CN", env.Config.I18n.DefaultLocale)

	// 验证基于配置文件初始化的 DB/Router/Auth 全链路可正常工作。
	tok := env.MustAuthUser("alice", "alice@example.com", "Password123!")
	rec := env.DoRequest(http.MethodGet, "/api/v1/me", testkit.WithBearer(tok.AccessToken))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNew_WithConfigFileAndOverrideOrderIndependent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	tomlContent := `
[app]
name = "base-from-file"
environment = "test"

[auth]
bcrypt_cost = 4

[database]
driver = "sqlite"
connection = ":memory:"
auto_migrate = true

[jwt]
secret = "file-secret-key-at-least-32-bytes-long"
access_token_expiry = "15m"
refresh_token_expiry = "24h"
issuer = "file-issuer"

[i18n]
default_locale = "en"
supported_locales = ["en", "zh-CN"]
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(tomlContent), 0644))

	// 即使 WithConfig 写在 WithConfigFile 之前，依然在配置文件加载后覆写。
	env := testkit.New(t,
		testkit.WithConfig(func(cfg *config.Config) {
			cfg.App.Name = "overridden-name"
		}),
		testkit.WithConfigFile(cfgPath),
	)

	assert.Equal(t, "overridden-name", env.Config.App.Name)
	assert.Equal(t, "file-issuer", env.Config.JWT.Issuer)
}

func TestNew_WithConfigFileDefaultSearch(t *testing.T) {
	t.Parallel()

	// 空路径自动通过 config.Load 查找项目 config/config.toml；
	// 配合 WithConfig 切到独立内存库，避免读写本地 dbs/main.db。
	env := testkit.New(t,
		testkit.WithConfigFile(""),
		testkit.WithConfig(func(cfg *config.Config) {
			cfg.Database = testkit.DefaultConfig().Database
			cfg.Auth.BcryptCost = testkit.DefaultConfig().Auth.BcryptCost
		}),
	)

	assert.Equal(t, "echo-template", env.Config.App.Name)
	assert.Equal(t, config.EnvLocal, env.Config.App.Environment)
	assert.Equal(t, "echo-template", env.Config.JWT.Issuer)
}
