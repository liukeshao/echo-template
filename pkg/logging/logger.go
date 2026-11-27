// Package logging provides a zero-dependency log/slog ContextHandler that
// automatically enriches log records with request_id from context.
package logging

import (
	"context"
	"log/slog"
	"os"
)

// ContextHandler 包装底层 slog.Handler，自动将 context 中的 request_id 写入日志属性。
type ContextHandler struct {
	slog.Handler
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestID := RequestID(ctx); requestID != "" {
		r = r.Clone()
		r.AddAttrs(slog.String("request_id", requestID))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithGroup(name)}
}

// NewContextHandler 包装原始 Handler。
func NewContextHandler(h slog.Handler) *ContextHandler {
	return &ContextHandler{Handler: h}
}

// Setup 以指定最低日志级别初始化全局 JSON slog.Logger（自动附带 ContextHandler）。
func Setup(level slog.Level) {
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(NewContextHandler(baseHandler)))
}
