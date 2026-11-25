package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const defaultInsecureJWTSecret = "your-super-secret-jwt-key-change-this-in-production"

type environment string

const (
	// EnvLocal represents the local environment.
	EnvLocal environment = "local"

	// EnvTest represents the test environment.
	EnvTest environment = "test"

	// EnvDevelop represents the development environment.
	EnvDevelop environment = "dev"

	// EnvStaging represents the staging environment.
	EnvStaging environment = "staging"

	// EnvQA represents the qa environment.
	EnvQA environment = "qa"

	// EnvProduction represents the production environment.
	EnvProduction environment = "prod"
)

type (
	// Config stores complete configuration.
	Config struct {
		HTTP     HTTPConfig
		App      AppConfig
		Database DatabaseConfig
		JWT      JWTConfig
		Clients  ClientsConfig
		I18n     I18nConfig
	}

	// I18nConfig 控制错误消息与字段校验消息的本地化行为。
	//
	// 加载的翻译文件位于 pkg/i18n/messages/<locale>.toml，通过 embed.FS 打包进二进制。
	// DefaultLocale 必须同时出现在 SupportedLocales 中，否则 i18n.NewTranslator 启动失败。
	I18nConfig struct {
		// DefaultLocale 为 fallback 语言；所有已注册错误码必须在该 locale 存在翻译。
		DefaultLocale string
		// SupportedLocales 列出服务端可返回的 locale，按偏好顺序排列。
		SupportedLocales []string
	}

	// HTTPConfig stores HTTP server configuration for rest.Server (*http.Server).
	HTTPConfig struct {
		Hostname string
		Port     uint16

		// GracefulTimeout controls how long rest.Server.Shutdown waits for
		// active requests to finish before closing connections.
		GracefulTimeout time.Duration

		// The following three fields are applied directly to the underlying net/http.Server.
		ReadTimeout  time.Duration
		WriteTimeout time.Duration
		IdleTimeout  time.Duration
	}

	// AppConfig stores application configuration.
	AppConfig struct {
		Name        string
		Host        string
		Environment environment
	}

	// JWTConfig stores JWT configuration.
	JWTConfig struct {
		Secret             string        // JWT签名密钥
		AccessTokenExpiry  time.Duration // Access token 过期时间
		RefreshTokenExpiry time.Duration // Refresh token 过期时间
		Issuer             string        // Token发行者
	}

	// DatabaseConfig stores the database configuration.
	DatabaseConfig struct {
		Driver      string
		Connection  string
		AutoMigrate bool
	}

	// ClientsConfig aggregates configuration for all third-party API clients.
	// Add a new field here for each external API your service integrates with.
	ClientsConfig struct {
		// Weather is the example client config; replace or extend as needed.
		Weather ExternalClientConfig
	}

	// ExternalClientConfig holds the common settings shared by every third-party
	// HTTP API client. Embed or alias this struct inside ClientsConfig when adding
	// a new integration.
	ExternalClientConfig struct {
		// BaseURL is the root URL of the external API (no trailing slash).
		BaseURL string
		// APIKey is the credential passed via the Authorization / X-API-Key header.
		APIKey string
		// Timeout overrides the shared *http.Client timeout for this API.
		// Zero means the shared default (10 s) is used.
		Timeout time.Duration
		// Retry is the maximum number of additional attempts on transient errors.
		// 0 = no retry, 1 = one retry (two total attempts), etc.
		Retry int
	}
)

// GetConfig loads and returns configuration.
func GetConfig() (Config, error) {
	var c Config
	v := viper.New()

	// Load the config file.
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(".")
	v.AddConfigPath("config")
	v.AddConfigPath("../config")
	v.AddConfigPath("../../config")

	// Load env variables.
	v.SetEnvPrefix("ECHO_TEMPLATE")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return c, err
	}

	if err := v.Unmarshal(&c); err != nil {
		return c, err
	}

	if err := c.Validate(); err != nil {
		return c, err
	}

	return c, nil
}

// Validate checks critical configuration invariants at startup (fail-fast).
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Database.Driver) == "" || strings.TrimSpace(c.Database.Connection) == "" {
		return fmt.Errorf("config: database.driver and database.connection are required")
	}
	if strings.TrimSpace(c.JWT.Secret) == "" {
		return fmt.Errorf("config: jwt.secret is required")
	}
	if c.App.Environment == EnvProduction {
		if c.JWT.Secret == defaultInsecureJWTSecret || len(c.JWT.Secret) < 32 {
			return fmt.Errorf("config: production requires a strong jwt.secret (at least 32 bytes and not the default placeholder)")
		}
	}
	if c.JWT.AccessTokenExpiry <= 0 || c.JWT.RefreshTokenExpiry <= 0 {
		return fmt.Errorf("config: jwt token expiries must be positive")
	}
	return nil
}
