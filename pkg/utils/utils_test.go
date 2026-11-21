package utils_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/utils"
)

func TestULID(t *testing.T) {
	t.Parallel()
	id1 := utils.GenerateULID()
	id2 := utils.GenerateULID()

	assert.Len(t, id1, 26)
	assert.NotEqual(t, id1, id2)
	assert.True(t, utils.IsValidULID(id1))
	assert.False(t, utils.IsValidULID(""))
	assert.False(t, utils.IsValidULID("not-a-ulid"))
}

func TestPassword(t *testing.T) {
	t.Parallel()
	hash, err := utils.HashPassword("s3cretPass")
	require.NoError(t, err)
	assert.NotEqual(t, "s3cretPass", hash)

	assert.NoError(t, utils.VerifyPassword(hash, "s3cretPass"))
	assert.Error(t, utils.VerifyPassword(hash, "wrongPass"))
}

func TestRetry_SuccessAndTransient(t *testing.T) {
	t.Parallel()

	// Succeeds immediately on first attempt.
	calls := 0
	err := utils.Retry(context.Background(), 3, func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)

	// Retries transient errors until success.
	calls = 0
	err = utils.Retry(context.Background(), 3, func() error {
		calls++
		if calls < 2 {
			return errors.New("temporary")
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, calls)

	// maxAttempts < 1 still executes once.
	calls = 0
	err = utils.Retry(context.Background(), 0, func() error {
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
	err := utils.Retry(context.Background(), 5, func() error {
		calls++
		return utils.Permanent(sentinel)
	})
	assert.ErrorIs(t, err, sentinel)
	assert.Equal(t, 1, calls)
	assert.Nil(t, utils.Permanent(nil))
}

func TestRetry_RespectsContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := utils.Retry(ctx, 5, func() error {
		calls++
		cancel()
		return errors.New("retryable")
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls)
}
