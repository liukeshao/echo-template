// Package httpclient provides a shared, tuned *http.Client with built-in
// per-API timeout and exponential-backoff retry transport for outbound API clients.
package httpclient

import (
	"net/http"
	"time"

	"github.com/samber/do/v2"
)

const defaultTimeout = 10 * time.Second

// Package registers the shared *Client into the DI container.
var Package = do.Package(
	do.Lazy(New),
)

// Options configures a derived [Client] built on top of the shared Transport.
type Options struct {
	// Timeout is the total request timeout, including all retry attempts.
	// Values <= 0 inherit the shared client's default timeout (10 s).
	Timeout time.Duration

	// Retry is the maximum number of additional attempts on transient errors
	// (network errors, HTTP 429/502/503/504) for safe/idempotent requests.
	// Values <= 0 disable retries.
	Retry int
}

// Client wraps *http.Client so it can be registered in the DI container.
// Third-party API clients receive the shared *Client via do.MustInvoke and call
// [Client.WithOptions] to derive a per-API client sharing the connection pool.
type Client struct{ *http.Client }

// New constructs a shared *Client with a cloned, tuned Transport and a
// conservative default timeout.
func New(_ do.Injector) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 20
	transport.IdleConnTimeout = 90 * time.Second

	return &Client{
		Client: &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
		},
	}, nil
}

// WithOptions derives a per-API *Client that reuses the underlying Transport
// (connection pool, TLS config) while applying per-API Timeout and Retry policy.
func (c *Client) WithOptions(opts Options) *Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = c.Timeout
	}

	rt := c.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
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
