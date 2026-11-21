package config

import (
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
}
