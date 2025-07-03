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
	}

	// HTTPConfig stores HTTP configuration.
	HTTPConfig struct {
		Hostname        string
		Port            uint16
		ReadTimeout     time.Duration
		WriteTimeout    time.Duration
		IdleTimeout     time.Duration
		ShutdownTimeout time.Duration
	}

	// AppConfig stores application configuration.
	AppConfig struct {
		Name        string
		Host        string
		Environment environment
		Timeout     time.Duration
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
	viper.SetEnvPrefix("echo-template")
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
