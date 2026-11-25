package infra_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/infra"
)

func TestRetry_SuccessAndTransient(t *testing.T) {
	t.Parallel()

	calls := 0
	err := infra.Retry(context.Background(), 3, func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)

	calls = 0
	err = infra.Retry(context.Background(), 3, func() error {
		calls++
		if calls < 2 {
			return errors.New("temporary")
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, calls)

	calls = 0
	err = infra.Retry(context.Background(), 0, func() error {
		calls++
		return errors.New("boom")
	})
	require.Error(t, err)
	assert.Equal(t, 1, calls)
}

func TestRetry_PermanentStopsImmediately(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("bad request")
	calls := 0
	err := infra.Retry(context.Background(), 5, func() error {
		calls++
		return infra.Permanent(sentinel)
	})
	assert.ErrorIs(t, err, sentinel)
	assert.Equal(t, 1, calls)
	assert.Nil(t, infra.Permanent(nil))
}

func TestRetry_RespectsContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := infra.Retry(ctx, 5, func() error {
		calls++
		cancel()
		return errors.New("retryable")
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls)
}
