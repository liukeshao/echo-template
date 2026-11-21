package weather_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/clients/weather"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/testutil"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClient_GetCurrent_EscapesCityAndParsesResponse(t *testing.T) {
	t.Parallel()
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

	env := testutil.New(t,
		testutil.WithConfig(func(cfg *config.Config) {
			cfg.Clients.Weather.APIKey = "secret-key"
		}),
		testutil.WithHTTPTransport(rt),
	)

	c := do.MustInvoke[*weather.Client](env.Injector)
	res, err := c.GetCurrent(context.Background(), "New York&evil=1")
	require.NoError(t, err)
	assert.Equal(t, "city=New+York%26evil%3D1", gotRawQuery)
	assert.Equal(t, "New York", res.City)
	assert.InDelta(t, 22.5, res.TempCelsius, 1e-9)
}

func TestClient_GetCurrent_RateLimited(t *testing.T) {
	t.Parallel()
	rt := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewBufferString("slow down")),
			Header:     make(http.Header),
		}, nil
	})

	env := testutil.New(t, testutil.WithHTTPTransport(rt))
	c := do.MustInvoke[*weather.Client](env.Injector)
	_, err := c.GetCurrent(context.Background(), "Tokyo")
	testutil.AssertOopsCode(t, err, codes.WeatherRateLimited)
}

func TestClient_GetCurrent_DoesNotRetryPermanent4xx(t *testing.T) {
	t.Parallel()
	calls := 0
	rt := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewBufferString("bad city")),
			Header:     make(http.Header),
		}, nil
	})

	env := testutil.New(t,
		testutil.WithConfig(func(cfg *config.Config) { cfg.Clients.Weather.Retry = 3 }),
		testutil.WithHTTPTransport(rt),
	)

	c := do.MustInvoke[*weather.Client](env.Injector)
	_, err := c.GetCurrent(context.Background(), "bad")
	assert.Equal(t, 1, calls, "4xx permanent errors must not be retried")
	testutil.AssertOopsCode(t, err, codes.WeatherInternal)
}

func TestClient_GetCurrent_RetriesTransient503(t *testing.T) {
	t.Parallel()
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

	env := testutil.New(t,
		testutil.WithConfig(func(cfg *config.Config) { cfg.Clients.Weather.Retry = 2 }),
		testutil.WithHTTPTransport(rt),
	)

	c := do.MustInvoke[*weather.Client](env.Injector)
	res, err := c.GetCurrent(context.Background(), "Tokyo")
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.Equal(t, "Tokyo", res.City)
}
