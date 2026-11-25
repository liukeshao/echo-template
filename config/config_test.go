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
		App: AppConfig{Environment: EnvLocal},
		Database: DatabaseConfig{
			Driver:     "sqlite",
			Connection: ":memory:",
		},
		JWT: JWTConfig{
			Secret:             defaultInsecureJWTSecret,
			AccessTokenExpiry:  time.Hour,
			RefreshTokenExpiry: 24 * time.Hour,
		},
	}
}

func TestConfig_Validate(t *testing.T) {
	cfg := validBaseConfig()
	require.NoError(t, cfg.Validate())

	// Production rejects default placeholder secret.
	prodDefault := validBaseConfig()
	prodDefault.App.Environment = EnvProduction
	assert.Error(t, prodDefault.Validate())

	// Production rejects short secret (< 32 bytes).
	prodShort := validBaseConfig()
	prodShort.App.Environment = EnvProduction
	prodShort.JWT.Secret = "short-secret"
	assert.Error(t, prodShort.Validate())

	// Production accepts strong secret (>= 32 bytes).
	prodStrong := validBaseConfig()
	prodStrong.App.Environment = EnvProduction
	prodStrong.JWT.Secret = strings.Repeat("k", 32)
	assert.NoError(t, prodStrong.Validate())

	// Missing database fields.
	noDB := validBaseConfig()
	noDB.Database.Driver = ""
	assert.Error(t, noDB.Validate())

	// Missing JWT secret.
	noSecret := validBaseConfig()
	noSecret.JWT.Secret = "   "
	assert.Error(t, noSecret.Validate())

	// Non-positive token expiry.
	badExpiry := validBaseConfig()
	badExpiry.JWT.AccessTokenExpiry = 0
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

[app]
name = "custom-app"
environment = "local"

[auth]
bcryptCost = 10

[database]
driver = "sqlite"
connection = ":memory:"
autoMigrate = true

[jwt]
secret = "local-secret"
accessTokenExpiry = "1h"
refreshTokenExpiry = "24h"
issuer = "custom-issuer"

[i18n]
defaultLocale = "en"
supportedLocales = ["en", "zh-CN"]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(tomlContent), 0644))
	t.Chdir(dir)
	t.Setenv("ECHO_TEMPLATE_JWT_SECRET", "env-overridden-secret")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, uint16(9000), cfg.HTTP.Port)
	assert.Equal(t, "custom-app", cfg.App.Name)
	assert.Equal(t, 10, cfg.Auth.BcryptCost)
	assert.Equal(t, "env-overridden-secret", cfg.JWT.Secret)
	assert.True(t, cfg.Database.AutoMigrate)
}
