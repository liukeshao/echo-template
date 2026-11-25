package httpclient_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/httpclient"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackingReadCloser struct {
	io.Reader
	closed atomic.Bool
}

func (c *trackingReadCloser) Close() error {
	c.closed.Store(true)
	return nil
}

func newTestClient(rt http.RoundTripper, opts httpclient.Options) *httpclient.Client {
	base := httpclient.New(httpclient.Config{
		Timeout:   5 * time.Second,
		Transport: rt,
	})
	return base.WithOptions(opts)
}

func TestWithOptions_TimeoutInheritanceAndNoNestedRetry(t *testing.T) {
	t.Parallel()
	var calls int
	base := httpclient.New(httpclient.Config{
		Timeout: 7 * time.Second,
		Retry:   2,
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(strings.NewReader("busy")),
				Header:     make(http.Header),
			}, nil
		}),
	})

	inherited := base.WithOptions(httpclient.Options{})
	assert.Equal(t, 7*time.Second, inherited.Timeout)

	// Retry: 0 on derived client disables retry even when base had Retry: 2.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)
	resp, err := inherited.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, 1, calls)

	overridden := base.WithOptions(httpclient.Options{Timeout: 2 * time.Second, Retry: 1})
	assert.Equal(t, 2*time.Second, overridden.Timeout)
	calls = 0
	resp, err = overridden.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, 2, calls, "derived Retry: 1 must not multiply with base Retry: 2")
}

func TestRetryTransport_RetriesTransientStatusAndClosesIntermediateBody(t *testing.T) {
	t.Parallel()
	var calls int
	firstBody := &trackingReadCloser{Reader: strings.NewReader("temporary outage")}

	c := newTestClient(roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       firstBody,
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	}), httpclient.Options{Retry: 2})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	resp, err := c.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, 2, calls)
	assert.True(t, firstBody.closed.Load(), "intermediate retry response body must be closed")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryTransport_ReturnsFinalResponseWhenExhausted(t *testing.T) {
	t.Parallel()
	var calls int

	c := newTestClient(roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(strings.NewReader("rate limited")),
			Header:     make(http.Header),
		}, nil
	}), httpclient.Options{Retry: 1})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	resp, err := c.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, 2, calls)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "rate limited", string(body))
}

func TestRetryTransport_DoesNotRetryNonTransientStatuses(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError} {
		var calls int
		c := newTestClient(roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(strings.NewReader("err")),
				Header:     make(http.Header),
			}, nil
		}), httpclient.Options{Retry: 3})

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
		require.NoError(t, err)

		resp, err := c.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, 1, calls, "status %d must not be retried", status)
	}
}

func TestRetryTransport_DoesNotRetryNonIdempotentMethodOrUnreplayableBody(t *testing.T) {
	t.Parallel()

	// POST is non-idempotent -> never retried.
	var postCalls int
	postClient := newTestClient(roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		postCalls++
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("busy")),
			Header:     make(http.Header),
		}, nil
	}), httpclient.Options{Retry: 2})

	postReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com", strings.NewReader(`{"x":1}`))
	require.NoError(t, err)
	resp, err := postClient.Do(postReq)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, 1, postCalls, "POST must not be retried automatically")

	// PUT with replayable body (GetBody != nil) -> retries with fresh body.
	var putBodies []string
	putClient := newTestClient(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, readErr := io.ReadAll(r.Body)
		require.NoError(t, readErr)
		putBodies = append(putBodies, string(b))
		if len(putBodies) == 1 {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("bad gw")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	}), httpclient.Options{Retry: 2})

	putReq, err := http.NewRequestWithContext(context.Background(), http.MethodPut, "https://example.com", bytes.NewReader([]byte("payload")))
	require.NoError(t, err)
	resp, err = putClient.Do(putReq)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, []string{"payload", "payload"}, putBodies)
}

func TestRetryTransport_RetriesNetworkErrorAndRespectsContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	var calls int

	c := newTestClient(roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls++
		cancel()
		return nil, errors.New("connection reset")
	}), httpclient.Options{Retry: 3})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	_, err = c.Do(req)
	require.Error(t, err)
	assert.Equal(t, 1, calls, "cancelled caller context must stop retries immediately")
}
