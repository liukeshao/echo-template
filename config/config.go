package config

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// EnvPrefix 是所有配置项环境变量覆盖的统一前缀（如 GO_REST_TEMPLATE_AUTH_JWT_SECRET）。
const EnvPrefix = "GO_REST_TEMPLATE"

const defaultInsecureJWTSecret = "your-super-secret-jwt-key-change-this-in-production"

type Environment string

const (
	EnvLocal      Environment = "local"
	EnvTest       Environment = "test"
	EnvDev        Environment = "dev"
	EnvStaging    Environment = "staging"
	EnvProduction Environment = "prod"
)

// Valid 判断环境标识是否为受支持的枚举值，避免拼写错误（如 "production"）绕过生产安全校验。
func (e Environment) Valid() bool {
	switch e {
	case EnvLocal, EnvTest, EnvDev, EnvStaging, EnvProduction:
		return true
	default:
		return false
	}
}

type (
	Config struct {
		HTTP     HTTP     `mapstructure:"http"`
		App      App      `mapstructure:"app"`
		Auth     Auth     `mapstructure:"auth"`
		Database Database `mapstructure:"database"`
		Clients  Clients  `mapstructure:"clients"`
		I18n     I18n     `mapstructure:"i18n"`
	}

	Auth struct {
		// BcryptCost 取值 4–31；为 0 时使用 service.DefaultBcryptCost（12）。
		BcryptCost int `mapstructure:"bcrypt_cost"`
		JWT        JWT `mapstructure:"jwt"`
	}

	// I18n 控制国际化配置；DefaultLocale 必须包含在 SupportedLocales 中。
	I18n struct {
		DefaultLocale    string   `mapstructure:"default_locale"`
		SupportedLocales []string `mapstructure:"supported_locales"`
	}

	HTTP struct {
		Hostname        string        `mapstructure:"hostname"`
		Port            uint16        `mapstructure:"port"`
		GracefulTimeout time.Duration `mapstructure:"graceful_timeout"`
		ReadTimeout     time.Duration `mapstructure:"read_timeout"`
		WriteTimeout    time.Duration `mapstructure:"write_timeout"`
		IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
		// MaxBodyBytes 限制单次请求体最大字节数；<= 0 时使用服务端默认值（2 MiB）。
		MaxBodyBytes int64 `mapstructure:"max_body_bytes"`
		CORS         CORS  `mapstructure:"cors"`
	}

	// CORS 配置跨域资源共享；AllowedOrigins 为空时默认不启用 CORS。
	CORS struct {
		AllowedOrigins   []string      `mapstructure:"allowed_origins"`
		AllowedMethods   []string      `mapstructure:"allowed_methods"`
		AllowedHeaders   []string      `mapstructure:"allowed_headers"`
		ExposedHeaders   []string      `mapstructure:"exposed_headers"`
		AllowCredentials bool          `mapstructure:"allow_credentials"`
		MaxAge           time.Duration `mapstructure:"max_age"`
	}

	App struct {
		Name        string      `mapstructure:"name"`
		Host        string      `mapstructure:"host"`
		Environment Environment `mapstructure:"environment"`
	}

	JWT struct {
		Secret             string        `mapstructure:"secret"`
		AccessTokenExpiry  time.Duration `mapstructure:"access_token_expiry"`
		RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
		Issuer             string        `mapstructure:"issuer"`
	}

	Database struct {
		Driver      string `mapstructure:"driver"`
		Connection  string `mapstructure:"connection"`
		AutoMigrate bool   `mapstructure:"auto_migrate"`
	}

	Clients struct {
		Weather ExternalClient `mapstructure:"weather"`
	}

	ExternalClient struct {
		BaseURL string        `mapstructure:"base_url"`
		APIKey  string        `mapstructure:"api_key"`
		Timeout time.Duration `mapstructure:"timeout"`
		Retry   int           `mapstructure:"retry"`
	}
)

// Load 加载默认搜索路径下的 config.toml、合并 EnvPrefix 前缀环境变量并执行校验。
func Load() (Config, error) {
	v := newViper()
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(".")
	v.AddConfigPath("config")
	v.AddConfigPath("../config")
	v.AddConfigPath("../../config")
	v.AddConfigPath("../../../config")
	return loadFromViper(v)
}

// LoadFile 从指定配置文件路径加载配置、合并 EnvPrefix 前缀环境变量并执行校验。
func LoadFile(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, fmt.Errorf("config: file path is required")
	}
	v := newViper()
	v.SetConfigFile(path)
	return loadFromViper(v)
}

func newViper() *viper.Viper {
	v := viper.New()
	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	return v
}

func loadFromViper(v *viper.Viper) (Config, error) {
	var c Config
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

// Validate 校验关键启动配置不变量。
func (c *Config) Validate() error {
	if !c.App.Environment.Valid() {
		return fmt.Errorf("config: invalid app.environment %q (must be one of: local, test, dev, staging, prod)", c.App.Environment)
	}
	if c.HTTP.GracefulTimeout < 0 || c.HTTP.ReadTimeout < 0 || c.HTTP.WriteTimeout < 0 || c.HTTP.IdleTimeout < 0 {
		return fmt.Errorf("config: http timeouts must be non-negative")
	}
	if c.HTTP.MaxBodyBytes < 0 {
		return fmt.Errorf("config: http.max_body_bytes must be non-negative")
	}
	if c.HTTP.CORS.AllowCredentials && slices.Contains(c.HTTP.CORS.AllowedOrigins, "*") {
		return fmt.Errorf("config: http.cors.allow_credentials cannot be enabled with wildcard origin \"*\"")
	}
	if c.HTTP.CORS.MaxAge < 0 {
		return fmt.Errorf("config: http.cors.max_age must be non-negative")
	}
	if strings.TrimSpace(c.Database.Driver) == "" || strings.TrimSpace(c.Database.Connection) == "" {
		return fmt.Errorf("config: database.driver and database.connection are required")
	}
	if strings.TrimSpace(c.Auth.JWT.Secret) == "" {
		return fmt.Errorf("config: auth.jwt.secret is required")
	}
	if c.App.Environment == EnvProduction {
		if c.Auth.JWT.Secret == defaultInsecureJWTSecret || len(c.Auth.JWT.Secret) < 32 {
			return fmt.Errorf("config: production requires a strong auth.jwt.secret (at least 32 bytes and not the default placeholder)")
		}
	}
	if c.Auth.JWT.AccessTokenExpiry <= 0 || c.Auth.JWT.RefreshTokenExpiry <= 0 {
		return fmt.Errorf("config: auth.jwt token expiries must be positive")
	}
	if c.Auth.BcryptCost != 0 && (c.Auth.BcryptCost < 4 || c.Auth.BcryptCost > 31) {
		return fmt.Errorf("config: auth.bcrypt_cost must be 0 (default) or between 4 and 31")
	}
	if c.Clients.Weather.Timeout < 0 || c.Clients.Weather.Retry < 0 {
		return fmt.Errorf("config: clients.weather timeout and retry must be non-negative")
	}
	return nil
}
