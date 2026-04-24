package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/infra"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// Client is the concrete implementation of [API].
// Obtain it via the DI container; do not construct it manually.
type Client struct {
	cfg  config.ExternalClientConfig
	http *http.Client
}

// NewWeatherClient constructs Client from the DI container.
// It wires:
//   - config.Config.Clients.Weather for baseURL / apiKey / timeout / retry
//   - *infra.HTTPClient as the underlying transport (shared connection pool)
func NewWeatherClient(i do.Injector) (*Client, error) {
	cfg := do.MustInvoke[*config.Config](i)
	shared := do.MustInvoke[*infra.HTTPClient](i)

	apiCfg := cfg.Clients.Weather

	// Build a per-client http.Client that inherits the shared Transport
	// (connection pool, TLS config) but applies the API-specific timeout.
	timeout := apiCfg.Timeout
	if timeout == 0 {
		timeout = shared.Timeout
	}

	return &Client{
		cfg: apiCfg,
		http: &http.Client{
			Timeout:   timeout,
			Transport: shared.Transport,
		},
	}, nil
}

// currentWeatherResponse is the raw JSON shape returned by the external API.
// Adjust field names to match the real API contract.
type currentWeatherResponse struct {
	City        string  `json:"city"`
	Description string  `json:"description"`
	TempCelsius float64 `json:"temp_celsius"`
	Humidity    int     `json:"humidity"`
}

// GetCurrent implements [API].
// It retries on transient errors up to cfg.Retry additional times.
func (c *Client) GetCurrent(ctx context.Context, city string) (*CurrentWeather, error) {
	const op = "GetCurrent"

	var result *CurrentWeather

	err := utils.Retry(ctx, c.cfg.Retry+1, func() error {
		var attemptErr error
		result, attemptErr = c.doGetCurrent(ctx, city)
		return attemptErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) doGetCurrent(ctx context.Context, city string) (*CurrentWeather, error) {
	const op = "GetCurrent"

	url := fmt.Sprintf("%s/current?city=%s", c.cfg.BaseURL, city)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, weatherInternal(err, op)
	}
	req.Header.Set("X-API-Key", c.cfg.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		// Covers network errors, context cancellation and timeouts.
		return nil, weatherUnavailable(err, op)
	}
	defer resp.Body.Close() //nolint:errcheck

	switch resp.StatusCode {
	case http.StatusOK:
		// happy path — decode below
	case http.StatusTooManyRequests:
		return nil, weatherRateLimited(op)
	case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		return nil, weatherUnavailable(
			fmt.Errorf("upstream returned %d", resp.StatusCode), op,
		)
	default:
		return nil, weatherInternal(
			fmt.Errorf("unexpected status %d", resp.StatusCode), op,
		)
	}

	var raw currentWeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, weatherInternal(err, op)
	}

	return &CurrentWeather{
		City:        raw.City,
		Description: raw.Description,
		TempCelsius: raw.TempCelsius,
		Humidity:    raw.Humidity,
	}, nil
}

// compile-time check that *Client implements API.
var _ API = (*Client)(nil)

// timeout returns the effective per-request timeout for logging / tracing.
func (c *Client) timeout() time.Duration { return c.http.Timeout }
