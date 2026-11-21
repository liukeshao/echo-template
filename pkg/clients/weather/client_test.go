package weather

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/samber/do/v2"
	"github.com/samber/oops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/infra"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClient_GetCurrent_EscapesCityAndParsesResponse(t *testing.T) {
	var gotRawQuery string
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotRawQuery = r.URL.RawQuery
		assert.Equal(t, "secret-key", r.Header.Get("X-API-Key"))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"city":"New York","description":"Sunny","temp_celsius":22.5,"humidity":50}`)),
			Header:     make(http.Header),
		}, nil
	})

	inj := do.New(Package)
	do.ProvideValue(inj, &config.Config{
		Clients: config.ClientsConfig{
			Weather: config.ExternalClientConfig{
				BaseURL: "https://weather.example.com/v1",
				APIKey:  "secret-key",
				Timeout: 2 * time.Second,
				Retry:   0,
			},
		},
	})
	do.ProvideValue(inj, &infra.HTTPClient{Client: &http.Client{Transport: rt}})

	c := do.MustInvoke[*Client](inj)
	res, err := c.GetCurrent(context.Background(), "New York&evil=1")
	require.NoError(t, err)
	assert.Equal(t, "city=New+York%26evil%3D1", gotRawQuery)
	assert.Equal(t, "New York", res.City)
	assert.InDelta(t, 22.5, res.TempCelsius, 1e-9)
}

func TestClient_GetCurrent_RateLimited(t *testing.T) {
	rt := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewBufferString("slow down")),
			Header:     make(http.Header),
		}, nil
	})

	inj := do.New(Package)
	do.ProvideValue(inj, &config.Config{
		Clients: config.ClientsConfig{
			Weather: config.ExternalClientConfig{
				BaseURL: "https://weather.example.com/v1",
				Retry:   0,
			},
		},
	})
	do.ProvideValue(inj, &infra.HTTPClient{Client: &http.Client{Transport: rt}})

	c := do.MustInvoke[*Client](inj)
	_, err := c.GetCurrent(context.Background(), "Tokyo")
	require.Error(t, err)
	var oe oops.OopsError
	require.ErrorAs(t, err, &oe)
	assert.Equal(t, codes.WeatherRateLimited.String(), oe.Code())
}

func TestClient_GetCurrent_DoesNotRetryPermanent4xx(t *testing.T) {
	calls := 0
	rt := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewBufferString("bad city")),
			Header:     make(http.Header),
		}, nil
	})

	inj := do.New(Package)
	do.ProvideValue(inj, &config.Config{
		Clients: config.ClientsConfig{
			Weather: config.ExternalClientConfig{
				BaseURL: "https://weather.example.com/v1",
				Retry:   3,
			},
		},
	})
	do.ProvideValue(inj, &infra.HTTPClient{Client: &http.Client{Transport: rt}})

	c := do.MustInvoke[*Client](inj)
	_, err := c.GetCurrent(context.Background(), "bad")
	require.Error(t, err)
	assert.Equal(t, 1, calls, "4xx permanent errors must not be retried")

	var oe oops.OopsError
	require.ErrorAs(t, err, &oe)
	assert.Equal(t, codes.WeatherInternal.String(), oe.Code())
}

func TestClient_GetCurrent_RetriesTransient503(t *testing.T) {
	calls := 0
	rt := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls++
		if calls < 2 {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(bytes.NewBufferString("unavailable")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"city":"Tokyo","description":"Clear","temp_celsius":20,"humidity":40}`)),
			Header:     make(http.Header),
		}, nil
	})

	inj := do.New(Package)
	do.ProvideValue(inj, &config.Config{
		Clients: config.ClientsConfig{
			Weather: config.ExternalClientConfig{
				BaseURL: "https://weather.example.com/v1",
				Retry:   2,
			},
		},
	})
	do.ProvideValue(inj, &infra.HTTPClient{Client: &http.Client{Transport: rt}})

	c := do.MustInvoke[*Client](inj)
	res, err := c.GetCurrent(context.Background(), "Tokyo")
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.Equal(t, "Tokyo", res.City)
}
