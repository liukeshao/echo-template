package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/httpclient"
)

// Client is the concrete implementation of [API].
// Obtain it via the DI container; do not construct it manually.
type Client struct {
	cfg  config.ExternalClient
	http *httpclient.Client
}

// NewClient constructs Client from the DI container.
// It wires:
//   - config.Config.Clients.Weather for baseURL / apiKey / timeout / retry
//   - *httpclient.Client via WithOptions (shared connection pool + per-API timeout & retry)
func NewClient(i do.Injector) (*Client, error) {
	cfg := do.MustInvoke[*config.Config](i)
	shared := do.MustInvoke[*httpclient.Client](i)

	apiCfg := cfg.Clients.Weather
	apiCfg.BaseURL = strings.TrimRight(apiCfg.BaseURL, "/")

	return &Client{
		cfg: apiCfg,
		http: shared.WithOptions(httpclient.Options{
			Timeout: apiCfg.Timeout,
			Retry:   apiCfg.Retry,
		}),
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
// Transient failures (network errors, 429, 502/503/504) are retried automatically
// by the underlying httpclient transport up to cfg.Retry additional times.
func (c *Client) GetCurrent(ctx context.Context, city string) (*CurrentWeather, error) {
	const op = "GetCurrent"

	reqURL := fmt.Sprintf("%s/current?city=%s", c.cfg.BaseURL, url.QueryEscape(city))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, weatherInternal(err, op)
	}
	req.Header.Set("X-API-Key", c.cfg.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		// If the caller's own context was cancelled or timed out, propagate it directly
		// (as 499 / 504) without misclassifying as upstream 503.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		// Covers upstream network errors and per-client HTTP timeouts.
		return nil, weatherUnavailable(err, op)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		// happy path — decode below
	case http.StatusTooManyRequests:
		return nil, weatherRateLimited(op, strings.TrimSpace(resp.Header.Get("Retry-After")))
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
