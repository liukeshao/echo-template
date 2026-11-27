package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/httpclient"
)

// Client 是 API 接口的具体实现，由 DI 容器构造并注入共享连接池与独立超时/重试配置。
type Client struct {
	cfg  config.ExternalClient
	http *httpclient.Client
}

// NewClient 从 DI 容器装配天气 API 客户端。
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

// GetCurrent 实现 API；瞬态网络错误与 HTTP 429/502/503/504 由底层 httpclient 自动按配置重试。
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

var _ API = (*Client)(nil)
