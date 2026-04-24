package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

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

	// HTTPConfig stores HTTP configuration.
	// Field names align with echo v5 StartConfig and net/http.Server.
	HTTPConfig struct {
		Hostname string
		Port     uint16

		// GracefulTimeout is passed to echo.StartConfig.GracefulTimeout.
		// It controls how long the server waits for active requests to finish
		// before shutting down.
		GracefulTimeout time.Duration

		// The following three fields are applied to the underlying http.Server
		// via echo.StartConfig.BeforeServeFunc.
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
		Driver     string
		Connection string
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

	// Load the config file.
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("config")
	viper.AddConfigPath("../config")
	viper.AddConfigPath("../../config")

	// Load env variables.
	viper.SetEnvPrefix("ECHO_TEMPLATE")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		return c, err
	}

	if err := viper.Unmarshal(&c); err != nil {
		return c, err
	}

	return c, nil
}
