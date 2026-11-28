// Package testkit 提供带独立内存 SQLite 与自动清理的测试 DI 环境，安全支持 t.Parallel()，
// 同时支持从配置文件初始化测试环境。
package testkit

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/internal/auth"
	"github.com/liukeshao/echo-template/internal/client"
	"github.com/liukeshao/echo-template/internal/database"
	"github.com/liukeshao/echo-template/internal/handler"
	"github.com/liukeshao/echo-template/internal/i18n"
	"github.com/liukeshao/echo-template/internal/server"
	"github.com/liukeshao/echo-template/internal/service"
	"github.com/liukeshao/echo-template/pkg/httpclient"
)

// Env 封装单个测试专用的 DI 容器与按需初始化的运行时依赖。
// 每个 Env 拥有独立的内存 SQLite 实例，安全支持 t.Parallel()。
type Env struct {
	t        testing.TB
	Injector *do.RootScope
	Config   *config.Config

	handlerOnce sync.Once
	handler     http.Handler
}

type options struct {
	useConfigFile bool
	configFile    string
	configFns     []func(*config.Config)
	overrides     []func(do.Injector)
}

// Option 自定义测试环境的配置或 DI 覆盖项。
type Option func(*options)

// WithConfigFile 使用配置文件初始化基础配置（经由 config.LoadFile / config.Load 合并
// ECHO_TEMPLATE_ 环境变量并执行校验），替代 DefaultConfig。
// 传入空字符串时按 config.Load 默认路径查找 config.toml。
// 可与 WithConfig 组合使用：WithConfig 回调始终在配置文件加载完成后执行。
func WithConfigFile(path string) Option {
	return func(o *options) {
		o.useConfigFile = true
		o.configFile = path
	}
}

// WithConfig 在容器解析任何依赖前修改测试配置（无论基于 DefaultConfig 还是 WithConfigFile）。
func WithConfig(fn func(*config.Config)) Option {
	return func(o *options) {
		o.configFns = append(o.configFns, fn)
	}
}

// WithOverride 用指定值（通常是 mock/stub）替换或注册 DI 容器中的类型 T。
func WithOverride[T any](val T) Option {
	return func(o *options) {
		o.overrides = append(o.overrides, func(i do.Injector) {
			do.Override(i, func(_ do.Injector) (T, error) { return val, nil })
		})
	}
}

// WithHTTPTransport 替换共享 *httpclient.Client 的 Transport，
// 用于在无网络环境下测试第三方 API 客户端。
func WithHTTPTransport(rt http.RoundTripper) Option {
	return WithOverride(httpclient.New(httpclient.Config{
		Timeout:   5 * time.Second,
		Transport: rt,
	}))
}

// WithInjector 提供底层 DI 容器回调，适合需要自定义 do.Bind 等高级装配的场景。
func WithInjector(fn func(do.Injector)) Option {
	return func(o *options) {
		o.overrides = append(o.overrides, fn)
	}
}

// DefaultConfig 返回测试专用的默认配置（独立内存 SQLite、测试 JWT、en + zh-CN 国际化）。
func DefaultConfig() config.Config {
	return config.Config{
		App: config.App{
			Name:        "echo-template-test",
			Host:        "http://localhost:8000",
			Environment: config.EnvTest,
		},
		Auth: config.Auth{
			BcryptCost: service.MinBcryptCost,
		},
		HTTP: config.HTTP{
			Hostname:        "127.0.0.1",
			Port:            8000,
			GracefulTimeout: time.Second,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    10 * time.Second,
			IdleTimeout:     time.Minute,
		},
		Database: config.Database{
			Driver:      "sqlite",
			Connection:  fmt.Sprintf("file:test_%s?mode=memory&cache=shared&_pragma=foreign_keys(1)", uuid.NewV7().String()),
			AutoMigrate: true,
		},
		JWT: config.JWT{
			Secret:             "test-secret-key-at-least-32-bytes-long",
			AccessTokenExpiry:  15 * time.Minute,
			RefreshTokenExpiry: 24 * time.Hour,
			Issuer:             "echo-template-test",
		},
		Clients: config.Clients{
			Weather: config.ExternalClient{
				BaseURL: "https://weather.example.com/v1",
				APIKey:  "test-api-key",
				Timeout: 2 * time.Second,
				Retry:   0,
			},
		},
		I18n: config.I18n{
			DefaultLocale:    "en",
			SupportedLocales: []string{"en", "zh-CN"},
		},
	}
}

// New 构造与 cmd/main.go 一致的完整 DI 容器（所有组件均为 Lazy 按需初始化），
// 并在测试结束时自动 Shutdown 容器释放资源。
func New(t testing.TB, opts ...Option) *Env {
	t.Helper()

	var o options
	for _, opt := range opts {
		opt(&o)
	}

	var cfg config.Config
	if o.useConfigFile {
		var err error
		if o.configFile == "" {
			cfg, err = config.Load()
		} else {
			cfg, err = config.LoadFile(o.configFile)
		}
		require.NoError(t, err)
	} else {
		cfg = DefaultConfig()
	}
	for _, fn := range o.configFns {
		fn(&cfg)
	}

	inj := do.New(
		database.Package,
		i18n.Package,
		auth.Package,
		client.Package,
		service.Package,
		handler.Package,
		server.Package,
	)
	do.ProvideValue(inj, &cfg)

	for _, apply := range o.overrides {
		apply(inj)
	}

	t.Cleanup(func() {
		_ = inj.Shutdown()
	})

	return &Env{
		t:        t,
		Injector: inj,
		Config:   &cfg,
	}
}

// ORM 按需解析并返回当前测试环境的 *ent.Client（用于准备测试数据或断言落库结果）。
func (e *Env) ORM() *ent.Client {
	e.t.Helper()
	return do.MustInvoke[*database.ORM](e.Injector).Client
}

// Handler 按需构建并返回已挂载全部路由、中间件与 RFC 9457 错误处理器的根 http.Handler。
func (e *Env) Handler() http.Handler {
	e.t.Helper()
	e.handlerOnce.Do(func() {
		h, err := server.BuildRouter(e.Injector)
		require.NoError(e.t, err)
		e.handler = h
	})
	return e.handler
}
