// Package httpclient provides a reusable, zero-dependency *http.Client wrapper
// with a tuned connection pool, per-client timeout, and exponential-backoff retry
// transport for safe/idempotent outbound HTTP requests.
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

// Config configures a [Client] created by [New].
// Zero-valued fields fall back to sensible production defaults.
type Config struct {
	// Timeout is the total request timeout, including all retry attempts.
	// Values <= 0 default to 10s.
	Timeout time.Duration

	// Retry is the maximum number of additional attempts on transient errors
	// (network errors, HTTP 429/502/503/504) for safe/idempotent requests.
	// Values <= 0 disable retries.
	Retry int

	// Connection pool settings used when Transport is nil.
	MaxIdleConns        int           // default: 100
	MaxIdleConnsPerHost int           // default: 20
	IdleConnTimeout     time.Duration // default: 90s

	// Transport optionally supplies a custom base http.RoundTripper
	// (e.g. for custom TLS settings or unit test stubs).
	// When nil, New clones http.DefaultTransport with the pool settings above.
	Transport http.RoundTripper
}

// Options configures a derived [Client] created by [Client.WithOptions] on top
// of an existing client's shared Transport.
type Options struct {
	// Timeout is the total request timeout, including all retry attempts.
	// Values <= 0 inherit the parent client's timeout.
	Timeout time.Duration

	// Retry is the maximum number of additional attempts on transient errors
	// (network errors, HTTP 429/502/503/504) for safe/idempotent requests.
	// Values <= 0 disable retries on the derived client.
	Retry int
}

// Client wraps *http.Client and supports deriving per-API clients that share
// the underlying connection pool via [Client.WithOptions].
type Client struct{ *http.Client }

// New constructs a *Client using cfg. Zero-valued fields use production defaults.
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

// WithOptions derives a per-API *Client that reuses the underlying base Transport
// (connection pool, TLS config) while applying per-API Timeout and Retry policy.
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

// DrainAndClose drains up to 4 KiB from an HTTP response body before closing it
// so the underlying TCP connection can be returned to the keep-alive pool.
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
