package utils

import (
	"context"
	"time"
)

const retryBaseDelay = 200 * time.Millisecond

// Retry calls fn up to maxAttempts times (must be ≥ 1).
// It retries whenever fn returns a non-nil error, using a linear back-off:
//
//	attempt 1 → no delay
//	attempt 2 → 200 ms
//	attempt 3 → 400 ms
//	…
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
			delay := time.Duration(attempt) * retryBaseDelay
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
	}
	return err
}
