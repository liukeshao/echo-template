package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validBaseConfig() Config {
	return Config{
		App: App{Environment: EnvLocal},
		Database: Database{
			Driver:     "sqlite",
			Connection: ":memory:",
		},
		Auth: Auth{
			JWT: JWT{
				Secret:             defaultInsecureJWTSecret,
				AccessTokenExpiry:  15 * time.Minute,
				RefreshTokenExpiry: 24 * time.Hour,
			},
		},
	}
}

func TestConfig_Validate(t *testing.T) {
	cfg := validBaseConfig()
	require.NoError(t, cfg.Validate())

	// Rejects unknown or mistyped environment (e.g. "production" instead of "prod").
	badEnv := validBaseConfig()
	badEnv.App.Environment = "production"
	assert.Error(t, badEnv.Validate())

	emptyEnv := validBaseConfig()
	emptyEnv.App.Environment = ""
	assert.Error(t, emptyEnv.Validate())

	// Rejects negative HTTP timeouts, negative max_body_bytes, or insecure CORS wildcard + credentials.
	negTimeout := validBaseConfig()
	negTimeout.HTTP.ReadTimeout = -time.Second
	assert.Error(t, negTimeout.Validate())

	negBody := validBaseConfig()
	negBody.HTTP.MaxBodyBytes = -1
	assert.Error(t, negBody.Validate())

	badCORS := validBaseConfig()
	badCORS.HTTP.CORS = CORS{AllowedOrigins: []string{"*"}, AllowCredentials: true}
	assert.Error(t, badCORS.Validate())

	// Production rejects default placeholder secret.
	prodDefault := validBaseConfig()
	prodDefault.App.Environment = EnvProduction
	assert.Error(t, prodDefault.Validate())

	// Production rejects short secret (< 32 bytes).
	prodShort := validBaseConfig()
	prodShort.App.Environment = EnvProduction
	prodShort.Auth.JWT.Secret = "short-secret"
	assert.Error(t, prodShort.Validate())

	// Production accepts strong secret (>= 32 bytes).
	prodStrong := validBaseConfig()
	prodStrong.App.Environment = EnvProduction
	prodStrong.Auth.JWT.Secret = strings.Repeat("k", 32)
	assert.NoError(t, prodStrong.Validate())

	// Missing database fields.
	noDB := validBaseConfig()
	noDB.Database.Driver = ""
	assert.Error(t, noDB.Validate())

	// Missing JWT secret.
	noSecret := validBaseConfig()
	noSecret.Auth.JWT.Secret = "   "
	assert.Error(t, noSecret.Validate())

	// Non-positive token expiry.
	badExpiry := validBaseConfig()
	badExpiry.Auth.JWT.AccessTokenExpiry = 0
	assert.Error(t, badExpiry.Validate())

	// Out-of-range bcrypt cost.
	lowCost := validBaseConfig()
	lowCost.Auth.BcryptCost = 3
	assert.Error(t, lowCost.Validate())

	highCost := validBaseConfig()
	highCost.Auth.BcryptCost = 32
	assert.Error(t, highCost.Validate())

	validCost := validBaseConfig()
	validCost.Auth.BcryptCost = 10
	assert.NoError(t, validCost.Validate())
}

func TestLoad_LoadsTOMLAndEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	tomlContent := `
[http]
port = 9000
graceful_timeout = "15s"
max_body_bytes = 4194304

[app]
name = "custom-app"
environment = "local"

[auth]
bcrypt_cost = 10

[auth.jwt]
secret = "local-secret"
access_token_expiry = "15m"
refresh_token_expiry = "24h"
issuer = "custom-issuer"

[database]
driver = "sqlite"
connection = ":memory:"
auto_migrate = true

[i18n]
default_locale = "en"
supported_locales = ["en", "zh-CN"]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(tomlContent), 0644))
	t.Chdir(dir)
	t.Setenv("GO_REST_TEMPLATE_AUTH_JWT_SECRET", "env-overridden-secret")
	t.Setenv("GO_REST_TEMPLATE_AUTH_JWT_ACCESS_TOKEN_EXPIRY", "30m")
	t.Setenv("GO_REST_TEMPLATE_I18N_DEFAULT_LOCALE", "zh-CN")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, uint16(9000), cfg.HTTP.Port)
	assert.Equal(t, 15*time.Second, cfg.HTTP.GracefulTimeout)
	assert.Equal(t, int64(4194304), cfg.HTTP.MaxBodyBytes)
	assert.Equal(t, "custom-app", cfg.App.Name)
	assert.Equal(t, 10, cfg.Auth.BcryptCost)
	assert.Equal(t, "env-overridden-secret", cfg.Auth.JWT.Secret)
	assert.Equal(t, 30*time.Minute, cfg.Auth.JWT.AccessTokenExpiry)
	assert.Equal(t, "zh-CN", cfg.I18n.DefaultLocale)
	assert.True(t, cfg.Database.AutoMigrate)
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.toml")
	tomlContent := `
[http]
port = 8088

[app]
name = "file-app"
environment = "test"

[auth]
bcrypt_cost = 4

[auth.jwt]
secret = "file-secret"
access_token_expiry = "30m"
refresh_token_expiry = "12h"
issuer = "file-issuer"

[database]
driver = "sqlite"
connection = ":memory:"
auto_migrate = true

[i18n]
default_locale = "en"
supported_locales = ["en", "zh-CN"]
`
	require.NoError(t, os.WriteFile(path, []byte(tomlContent), 0644))
	t.Setenv("GO_REST_TEMPLATE_APP_NAME", "env-file-app")

	cfg, err := LoadFile(path)
	require.NoError(t, err)
	assert.Equal(t, uint16(8088), cfg.HTTP.Port)
	assert.Equal(t, "env-file-app", cfg.App.Name)
	assert.Equal(t, EnvTest, cfg.App.Environment)
	assert.Equal(t, 4, cfg.Auth.BcryptCost)
	assert.Equal(t, "file-secret", cfg.Auth.JWT.Secret)
	assert.Equal(t, 30*time.Minute, cfg.Auth.JWT.AccessTokenExpiry)

	_, err = LoadFile("")
	assert.Error(t, err)

	_, err = LoadFile(filepath.Join(dir, "missing.toml"))
	assert.Error(t, err)
}
