// Package httpclient 提供零外部依赖的出站 HTTP 客户端封装，支持连接池复用、独立超时与幂等指数退避重试。
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

// Config 配置出站 HTTP 客户端，零值字段回落到默认生产参数。
type Config struct {
	Timeout             time.Duration
	Retry               int
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
	Transport           http.RoundTripper
}

// Options 配置派生子客户端的超时与重试策略（Timeout <= 0 时继承父客户端）。
type Options struct {
	Timeout time.Duration
	Retry   int
}

type Client struct{ *http.Client }

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

// WithOptions 共享底层连接池，派生具备独立超时与重试策略的子客户端。
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
