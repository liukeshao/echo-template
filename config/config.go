package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const defaultInsecureJWTSecret = "your-super-secret-jwt-key-change-this-in-production"

// Environment represents the application runtime environment.
type Environment string

const (
	// EnvLocal represents the local environment.
	EnvLocal Environment = "local"

	// EnvTest represents the test environment.
	EnvTest Environment = "test"

	// EnvDev represents the development environment.
	EnvDev Environment = "dev"

	// EnvStaging represents the staging environment.
	EnvStaging Environment = "staging"

	// EnvQA represents the qa environment.
	EnvQA Environment = "qa"

	// EnvProduction represents the production environment.
	EnvProduction Environment = "prod"
)

type (
	// Config stores complete configuration.
	Config struct {
		HTTP     HTTP     `mapstructure:"http"`
		App      App      `mapstructure:"app"`
		Auth     Auth     `mapstructure:"auth"`
		Database Database `mapstructure:"database"`
		JWT      JWT      `mapstructure:"jwt"`
		Clients  Clients  `mapstructure:"clients"`
		I18n     I18n     `mapstructure:"i18n"`
	}

	// Auth stores authentication policy settings.
	Auth struct {
		// BcryptCost sets the bcrypt hashing cost (4–31).
		// Zero uses service.DefaultBcryptCost (12).
		BcryptCost int `mapstructure:"bcrypt_cost"`
	}

	// I18n 控制错误消息与字段校验消息的本地化行为。
	//
	// 加载的翻译文件位于 internal/i18n/messages/<locale>.toml，通过 embed.FS 打包进二进制。
	// DefaultLocale 必须同时出现在 SupportedLocales 中，否则 i18n.NewTranslator 启动失败。
	I18n struct {
		// DefaultLocale 为 fallback 语言；所有已注册错误码必须在该 locale 存在翻译。
		DefaultLocale string `mapstructure:"default_locale"`
		// SupportedLocales 列出服务端可返回的 locale，按偏好顺序排列。
		SupportedLocales []string `mapstructure:"supported_locales"`
	}

	// HTTP stores HTTP server configuration for httpkit.Server (*http.Server).
	HTTP struct {
		Hostname string `mapstructure:"hostname"`
		Port     uint16 `mapstructure:"port"`

		// GracefulTimeout controls how long httpkit.Server.Shutdown waits for
		// active requests to finish before closing connections.
		GracefulTimeout time.Duration `mapstructure:"graceful_timeout"`

		// The following three fields are applied directly to the underlying net/http.Server.
		ReadTimeout  time.Duration `mapstructure:"read_timeout"`
		WriteTimeout time.Duration `mapstructure:"write_timeout"`
		IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	}

	// App stores application configuration.
	App struct {
		Name        string      `mapstructure:"name"`
		Host        string      `mapstructure:"host"`
		Environment Environment `mapstructure:"environment"`
	}

	// JWT stores JWT configuration.
	JWT struct {
		Secret             string        `mapstructure:"secret"`               // JWT签名密钥
		AccessTokenExpiry  time.Duration `mapstructure:"access_token_expiry"`  // Access token 过期时间
		RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"` // Refresh token 过期时间
		Issuer             string        `mapstructure:"issuer"`               // Token发行者
	}

	// Database stores the database configuration.
	Database struct {
		Driver      string `mapstructure:"driver"`
		Connection  string `mapstructure:"connection"`
		AutoMigrate bool   `mapstructure:"auto_migrate"`
	}

	// Clients aggregates configuration for all third-party API clients.
	// Add a new field here for each external API your service integrates with.
	Clients struct {
		// Weather is the example client config; replace or extend as needed.
		Weather ExternalClient `mapstructure:"weather"`
	}

	// ExternalClient holds the common settings shared by every third-party
	// HTTP API client. Embed or alias this struct inside Clients when adding
	// a new integration.
	ExternalClient struct {
		// BaseURL is the root URL of the external API (no trailing slash).
		BaseURL string `mapstructure:"base_url"`
		// APIKey is the credential passed via the Authorization / X-API-Key header.
		APIKey string `mapstructure:"api_key"`
		// Timeout sets the total request timeout (including retries) for this API.
		// Zero inherits the shared default (10 s).
		Timeout time.Duration `mapstructure:"timeout"`
		// Retry is the maximum number of additional attempts on transient errors
		// (network errors, HTTP 429/502/503/504) for idempotent requests, enforced
		// automatically by httpclient.Client.WithOptions.
		// 0 = no retry, 1 = one retry (two total attempts), etc.
		Retry int `mapstructure:"retry"`
	}
)

// Load loads, validates, and returns configuration.
func Load() (Config, error) {
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
	if c.Auth.BcryptCost != 0 && (c.Auth.BcryptCost < 4 || c.Auth.BcryptCost > 31) {
		return fmt.Errorf("config: auth.bcrypt_cost must be 0 (default) or between 4 and 31")
	}
	return nil
}
