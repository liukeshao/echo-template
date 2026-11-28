package httpclient

import (
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	retryBaseDelay = 200 * time.Millisecond
	retryMaxDelay  = 5 * time.Second
	maxRetryAfter  = 30 * time.Second
)

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
		resp      *http.Response
		err       error
		nextDelay time.Duration
		hasDelay  bool
	)
	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			delay := backoffFn(attempt)
			if hasDelay {
				delay = nextDelay
				hasDelay = false
			}
			if err := sleepContext(req, delay); err != nil {
				return nil, err
			}
		}

		attemptReq, prepErr := prepareAttemptRequest(req, attempt)
		if prepErr != nil {
			return nil, prepErr
		}

		resp, err = t.base.RoundTrip(attemptReq)
		if err != nil {
			if req.Context().Err() != nil {
				return nil, err
			}
			continue
		}

		if !isRetryableStatus(resp.StatusCode) || attempt == t.maxRetries {
			return resp, nil
		}

		if d, ok := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
			nextDelay = min(max(d, 0), maxRetryAfter)
			hasDelay = true
		}

		DrainAndClose(resp.Body)
	}

	return resp, err
}

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

// parseRetryAfter 解析标准 HTTP Retry-After 响应头（支持非负整数秒与 RFC 9110 HTTP-date 两种格式）。
func parseRetryAfter(raw string, now time.Time) (time.Duration, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if secs, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(raw); err == nil {
		return t.Sub(now), true
	}
	return 0, false
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

// backoffWithJitter 计算指数退避（200ms * 2^(attempt-1)，上限 5s）并叠加 [0.5, 1.0] 均匀抖动。
func backoffWithJitter(attempt int) time.Duration {
	shift := min(attempt-1, 6)
	base := min(retryBaseDelay<<shift, retryMaxDelay)
	half := base / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}
