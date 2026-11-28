package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const defaultInsecureJWTSecret = "your-super-secret-jwt-key-change-this-in-production"

type Environment string

const (
	EnvLocal      Environment = "local"
	EnvTest       Environment = "test"
	EnvDev        Environment = "dev"
	EnvStaging    Environment = "staging"
	EnvProduction Environment = "prod"
)

type (
	Config struct {
		HTTP     HTTP     `mapstructure:"http"`
		App      App      `mapstructure:"app"`
		Auth     Auth     `mapstructure:"auth"`
		Database Database `mapstructure:"database"`
		JWT      JWT      `mapstructure:"jwt"`
		Clients  Clients  `mapstructure:"clients"`
		I18n     I18n     `mapstructure:"i18n"`
	}

	Auth struct {
		// BcryptCost 取值 4–31；为 0 时使用 service.DefaultBcryptCost（12）。
		BcryptCost int `mapstructure:"bcrypt_cost"`
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

// Load 加载默认搜索路径下的 config.toml、合并 ECHO_TEMPLATE_ 前缀环境变量并执行校验。
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

// LoadFile 从指定配置文件路径加载配置、合并 ECHO_TEMPLATE_ 前缀环境变量并执行校验。
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
	v.SetEnvPrefix("ECHO_TEMPLATE")
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
