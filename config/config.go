package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const defaultInsecureJWTSecret = "your-super-secret-jwt-key-change-this-in-production"

// Environment 表示应用运行环境。
type Environment string

const (
	// EnvLocal 表示本地开发环境。
	EnvLocal Environment = "local"
	// EnvTest 表示自动化测试环境。
	EnvTest Environment = "test"
	// EnvDev 表示共享开发环境。
	EnvDev Environment = "dev"
	// EnvStaging 表示预发环境。
	EnvStaging Environment = "staging"
	// EnvProduction 表示生产环境。
	EnvProduction Environment = "prod"
)

type (
	// Config 保存完整的应用程序配置。
	Config struct {
		HTTP     HTTP     `mapstructure:"http"`
		App      App      `mapstructure:"app"`
		Auth     Auth     `mapstructure:"auth"`
		Database Database `mapstructure:"database"`
		JWT      JWT      `mapstructure:"jwt"`
		Clients  Clients  `mapstructure:"clients"`
		I18n     I18n     `mapstructure:"i18n"`
	}

	// Auth 保存认证策略配置。
	Auth struct {
		// BcryptCost 设置 bcrypt 哈希成本（4–31）；为 0 时使用 service.DefaultBcryptCost（12）。
		BcryptCost int `mapstructure:"bcrypt_cost"`
	}

	// I18n 控制错误消息与字段校验消息的本地化行为。
	//
	// 翻译文件位于 internal/i18n/messages/<locale>.toml，通过 embed.FS 打包进二进制。
	// DefaultLocale 必须同时出现在 SupportedLocales 中，否则 i18n.NewTranslator 启动失败。
	I18n struct {
		// DefaultLocale 为兜底语言；所有已注册状态码与校验规则必须在该 locale 存在翻译。
		DefaultLocale string `mapstructure:"default_locale"`
		// SupportedLocales 列出服务端可返回的 locale，按偏好顺序排列。
		SupportedLocales []string `mapstructure:"supported_locales"`
	}

	// HTTP 保存 httpkit.Server（*http.Server）配置。
	HTTP struct {
		Hostname string `mapstructure:"hostname"`
		Port     uint16 `mapstructure:"port"`

		// GracefulTimeout 控制优雅停机等待在途请求完成的最大时长。
		GracefulTimeout time.Duration `mapstructure:"graceful_timeout"`

		// 以下超时参数直接透传到底层 net/http.Server。
		ReadTimeout  time.Duration `mapstructure:"read_timeout"`
		WriteTimeout time.Duration `mapstructure:"write_timeout"`
		IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	}

	// App 保存应用基础信息配置。
	App struct {
		Name        string      `mapstructure:"name"`
		Host        string      `mapstructure:"host"`
		Environment Environment `mapstructure:"environment"`
	}

	// JWT 保存 JWT 签发与校验配置。
	JWT struct {
		Secret             string        `mapstructure:"secret"`
		AccessTokenExpiry  time.Duration `mapstructure:"access_token_expiry"`
		RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
		Issuer             string        `mapstructure:"issuer"`
	}

	// Database 保存数据库连接与迁移配置。
	Database struct {
		Driver      string `mapstructure:"driver"`
		Connection  string `mapstructure:"connection"`
		AutoMigrate bool   `mapstructure:"auto_migrate"`
	}

	// Clients 聚合全部第三方外部 API 客户端配置；新增外部集成时在此添加字段。
	Clients struct {
		Weather ExternalClient `mapstructure:"weather"`
	}

	// ExternalClient 保存第三方 HTTP API 客户端的通用配置项。
	ExternalClient struct {
		// BaseURL 为外部 API 根地址（不含末尾斜杠）。
		BaseURL string `mapstructure:"base_url"`
		// APIKey 为外部 API 鉴权凭证。
		APIKey string `mapstructure:"api_key"`
		// Timeout 为单次请求总超时（含重试）；为 0 时继承共享客户端默认值（10s）。
		Timeout time.Duration `mapstructure:"timeout"`
		// Retry 为幂等请求遇到瞬态错误（网络错误、429/502/503/504）时的最大额外重试次数。
		Retry int `mapstructure:"retry"`
	}
)

// Load 加载 config.toml、合并 ECHO_TEMPLATE_ 前缀环境变量并执行启动校验。
func Load() (Config, error) {
	var c Config
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(".")
	v.AddConfigPath("config")
	v.AddConfigPath("../config")
	v.AddConfigPath("../../config")

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

// Validate 在启动阶段校验关键配置不变量（fail-fast）。
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
