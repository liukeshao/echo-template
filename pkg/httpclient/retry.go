package httpclient

import (
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

const (
	retryBaseDelay = 200 * time.Millisecond
	retryMaxDelay  = 5 * time.Second
)

// retryTransport is an http.RoundTripper that retries idempotent requests on
// transient transport failures and HTTP 429/502/503/504 responses using
// exponential backoff with jitter.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
	backoff    func(attempt int) time.Duration
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.maxRetries <= 0 || !canRetryRequest(req) {
		return t.base.RoundTrip(req)
	}

	backoffFn := t.backoff
	if backoffFn == nil {
		backoffFn = backoffWithJitter
	}

	var (
		resp *http.Response
		err  error
	)
	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepContext(req, backoffFn(attempt)); err != nil {
				return nil, err
			}
		}

		attemptReq, prepErr := prepareAttemptRequest(req, attempt)
		if prepErr != nil {
			return nil, prepErr
		}

		resp, err = t.base.RoundTrip(attemptReq)
		if err != nil {
			// Stop immediately if the caller's context was cancelled or timed out.
			if req.Context().Err() != nil {
				return nil, err
			}
			continue
		}

		if !isRetryableStatus(resp.StatusCode) || attempt == t.maxRetries {
			return resp, nil
		}

		// Drain and close intermediate error response bodies so the underlying
		// TCP connection can be reused on the next attempt.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
	}

	return resp, err
}

// canRetryRequest reports whether req can be safely retried without risking
// duplicate side effects or consuming an unreplayable body.
func canRetryRequest(req *http.Request) bool {
	if !isIdempotentMethod(req.Method) {
		return false
	}
	if req.Body == nil || req.Body == http.NoBody {
		return true
	}
	return req.GetBody != nil
}

func isIdempotentMethod(method string) bool {
	switch method {
	case "", http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func prepareAttemptRequest(req *http.Request, attempt int) (*http.Request, error) {
	if attempt == 0 {
		return req, nil
	}
	cloned := req.Clone(req.Context())
	if req.Body != nil && req.Body != http.NoBody {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		cloned.Body = body
	}
	return cloned, nil
}

func sleepContext(req *http.Request, d time.Duration) error {
	if d <= 0 {
		return req.Context().Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-req.Context().Done():
		return req.Context().Err()
	case <-timer.C:
		return nil
	}
}

// backoffWithJitter computes exponential backoff (200ms * 2^(attempt-1)) capped
// at retryMaxDelay, multiplied by uniform jitter in [0.5, 1.0].
func backoffWithJitter(attempt int) time.Duration {
	shift := min(attempt-1, 6)
	base := min(retryBaseDelay<<shift, retryMaxDelay)
	// Jitter in [base/2, base] to spread out concurrent retries.
	half := base / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}
