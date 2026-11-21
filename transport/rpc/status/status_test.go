package status_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/transport/rpc/codes"
	"github.com/liukeshao/echo-template/transport/rpc/errdetails"
	"github.com/liukeshao/echo-template/transport/rpc/status"
)

func TestStatus_ImmutableAndDetailsCopy(t *testing.T) {
	t.Parallel()
	base := status.New(codes.NotFound, "missing")
	assert.Equal(t, codes.NotFound, base.Code())
	assert.Equal(t, "missing", base.Message())
	assert.Nil(t, base.Details())
	assert.Contains(t, base.Error(), "NOT_FOUND")

	detail := errdetails.ErrorInfo{Type: errdetails.TypeErrorInfo, Reason: "BOOK_NOT_FOUND"}
	withD := base.WithDetails(detail)
	assert.Nil(t, base.Details(), "original status must remain immutable")
	require.Len(t, withD.Details(), 1)

	// Mutating returned slice must not mutate internal state.
	got := withD.Details()
	got[0] = "tampered"
	assert.Equal(t, detail, withD.Details()[0])
}

func TestStatus_As(t *testing.T) {
	t.Parallel()
	s := status.New(codes.PermissionDenied, "denied")
	wrapped := fmt.Errorf("wrap: %w", s)

	found, ok := status.As(wrapped)
	require.True(t, ok)
	assert.Equal(t, s, found)

	_, ok = status.As(errors.New("plain"))
	assert.False(t, ok)
}

func TestStatus_FromContextError(t *testing.T) {
	t.Parallel()
	assert.Equal(t, codes.OK, status.FromContextError(nil).Code())
	assert.Equal(t, codes.DeadlineExceeded, status.FromContextError(context.DeadlineExceeded).Code())
	assert.Equal(t, codes.Canceled, status.FromContextError(context.Canceled).Code())
	assert.Equal(t, codes.Unknown, status.FromContextError(errors.New("other")).Code())
}
