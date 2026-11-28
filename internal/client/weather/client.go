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

	"github.com/liukeshao/go-rest-template/config"
	"github.com/liukeshao/go-rest-template/pkg/httpclient"
)

const maxResponseBytes = 1 << 20 // 1 MiB

type Client struct {
	cfg  config.ExternalClient
	http *httpclient.Client
}

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

type currentWeatherResponse struct {
	City        string  `json:"city"`
	Description string  `json:"description"`
	TempCelsius float64 `json:"temp_celsius"`
	Humidity    int     `json:"humidity"`
}

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
		// 调用方自身 context 已取消/超时时直接透传（出口归类为 499/504），不误判为上游 503。
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, weatherUnavailable(err, op)
	}
	defer httpclient.DrainAndClose(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
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
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&raw); err != nil {
		return nil, weatherInternal(err, op)
	}

	return &CurrentWeather{
		City:        raw.City,
		Description: raw.Description,
		TempCelsius: raw.TempCelsius,
		Humidity:    raw.Humidity,
	}, nil
}

var _ API = (*Client)(nil)
