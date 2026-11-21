package infra

import (
	"net/http"
	"time"

	"github.com/samber/do/v2"
)

const defaultHTTPTimeout = 10 * time.Second

// HTTPClient wraps *http.Client so it can be registered in the DI container.
// Third-party API clients receive this via do.MustInvoke and may build their
// own *http.Client with a per-API timeout on top of the shared Transport.
type HTTPClient struct{ *http.Client }

// NewHTTPClient constructs a shared *http.Client with a cloned, tuned Transport
// and conservative default timeout. Individual API clients should override the
// timeout via their own config when calling external services with tighter SLAs.
func NewHTTPClient(_ do.Injector) (*HTTPClient, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 20
	transport.IdleConnTimeout = 90 * time.Second

	return &HTTPClient{
		Client: &http.Client{
			Timeout:   defaultHTTPTimeout,
			Transport: transport,
		},
	}, nil
}
