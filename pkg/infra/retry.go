package infra

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"
)

const (
	retryBaseDelay = 200 * time.Millisecond
	retryMaxDelay  = 5 * time.Second
)

type permanentError struct {
	err error
}

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Permanent wraps err so Retry stops immediately without further attempts
// and returns the underlying error unwrapped.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

// Retry calls fn up to maxAttempts times (must be ≥ 1).
// It retries on transient errors using exponential back-off with jitter
// (capped at 5s). Errors wrapped with [Permanent] stop retries immediately.
//
// The loop stops early and returns ctx.Err() if the context is cancelled or
// its deadline is exceeded between attempts.
func Retry(ctx context.Context, maxAttempts int, fn func() error) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var err error
	for attempt := range maxAttempts {
		if attempt > 0 {
			delay := backoffWithJitter(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err = fn()
		if err == nil {
			return nil
		}
		if perm, ok := errors.AsType[*permanentError](err); ok {
			return perm.err
		}
	}
	return err
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
