package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextHandler_PreservesRequestIDAcrossWithAttrsAndWithGroup(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(NewContextHandler(base)).With("component", "test").WithGroup("sub")

	ctx := WithRequestID(context.Background(), "req-abc-123")
	logger.InfoContext(ctx, "hello", "k", "v")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "test", entry["component"])
	sub, ok := entry["sub"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "req-abc-123", sub["request_id"])
	assert.Equal(t, "v", sub["k"])
}

func TestSetup_LogLevels(t *testing.T) {
	orig := slog.Default()
	t.Cleanup(func() { slog.SetDefault(orig) })

	Setup(slog.LevelInfo)
	assert.False(t, slog.Default().Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, slog.Default().Enabled(context.Background(), slog.LevelInfo))

	Setup(slog.LevelDebug)
	assert.True(t, slog.Default().Enabled(context.Background(), slog.LevelDebug))
}

func TestRequestIDContext(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-123-456")

	got, ok := LookupRequestID(ctx)
	assert.True(t, ok)
	assert.Equal(t, "req-123-456", got)
	assert.Equal(t, "req-123-456", RequestID(ctx))
	assert.Empty(t, RequestID(context.Background()))
}
