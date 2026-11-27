// Package httpclient 提供零外部依赖的可复用出站 HTTP 客户端封装，
// 内置连接池调优、单客户端超时控制以及面向幂等请求的指数退避带抖动重试。
package httpclient

import (
	"io"
	"net/http"
	"time"
)

const (
	defaultTimeout             = 10 * time.Second
	defaultMaxIdleConns        = 100
	defaultMaxIdleConnsPerHost = 20
	defaultIdleConnTimeout     = 90 * time.Second
)

// Config 配置由 New 构造的 Client；零值字段自动回落到生产默认值。
type Config struct {
	// Timeout 为单次请求总超时（含所有重试）；<= 0 默认 10s。
	Timeout time.Duration
	// Retry 为幂等请求遇到瞬态错误（网络错误、429/502/503/504）的最大额外重试次数；<= 0 关闭重试。
	Retry int
	// 以下连接池参数在 Transport 为 nil 时生效。
	MaxIdleConns        int           // 默认 100
	MaxIdleConnsPerHost int           // 默认 20
	IdleConnTimeout     time.Duration // 默认 90s
	// Transport 可选注入自定义底层 RoundTripper（如 TLS 定制或单测 stub）。
	Transport http.RoundTripper
}

// Options 配置通过 Client.WithOptions 派生的子客户端（共享底层连接池）。
type Options struct {
	// Timeout 为单次请求总超时（含重试）；<= 0 继承父客户端超时。
	Timeout time.Duration
	// Retry 为最大额外重试次数；<= 0 关闭重试。
	Retry int
}

// Client 封装标准库 *http.Client，支持按第三方 API 派生独立超时与重试策略并复用连接池。
type Client struct{ *http.Client }

// New 根据 cfg 构造 *Client，零值字段自动应用生产默认值。
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	rt := cfg.Transport
	if rt == nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.MaxIdleConns = positiveOrDefault(cfg.MaxIdleConns, defaultMaxIdleConns)
		tr.MaxIdleConnsPerHost = positiveOrDefault(cfg.MaxIdleConnsPerHost, defaultMaxIdleConnsPerHost)
		tr.IdleConnTimeout = positiveOrDefault(cfg.IdleConnTimeout, defaultIdleConnTimeout)
		rt = tr
	}
	if cfg.Retry > 0 {
		rt = &retryTransport{
			base:       rt,
			maxRetries: cfg.Retry,
			backoff:    backoffWithJitter,
		}
	}

	return &Client{
		Client: &http.Client{
			Timeout:   timeout,
			Transport: rt,
		},
	}
}

// WithOptions 复用底层连接池与 TLS 配置，派生具备独立 Timeout 与 Retry 策略的 *Client。
func (c *Client) WithOptions(opts Options) *Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = c.Timeout
	}

	rt := baseTransport(c.Transport)
	if opts.Retry > 0 {
		rt = &retryTransport{
			base:       rt,
			maxRetries: opts.Retry,
			backoff:    backoffWithJitter,
		}
	}

	return &Client{
		Client: &http.Client{
			Timeout:   timeout,
			Transport: rt,
		},
	}
}

func baseTransport(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		return http.DefaultTransport
	}
	if r, ok := rt.(*retryTransport); ok {
		return r.base
	}
	return rt
}

// DrainAndClose 排空响应体最多 4 KiB 后关闭，确保底层 TCP 连接可安全归还 keep-alive 连接池。
func DrainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 4096))
	_ = body.Close()
}

func positiveOrDefault[T ~int | ~int64](v, fallback T) T {
	if v <= 0 {
		return fallback
	}
	return v
}
