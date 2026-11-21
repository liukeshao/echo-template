package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/appctx"
)

func TestContextHandler_PreservesRequestIDAcrossWithAttrsAndWithGroup(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(NewContextHandler(base)).With("component", "test").WithGroup("sub")

	ctx := appctx.WithRequestID(context.Background(), "req-abc-123")
	logger.InfoContext(ctx, "hello", "k", "v")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "test", entry["component"])
	sub, ok := entry["sub"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "req-abc-123", sub["request_id"])
	assert.Equal(t, "v", sub["k"])
}
