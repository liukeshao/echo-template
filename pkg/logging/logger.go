// Package logging 提供零依赖的 slog ContextHandler，自动向日志顶层注入 context 中的 request_id。
package logging

import (
	"context"
	"log/slog"
	"os"
	"slices"
)

// Format 指定默认日志输出格式。
type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

type handlerOp struct {
	group string
	attrs []slog.Attr
}

// ContextHandler 包装任意 slog.Handler，在 Handle 时自动将 context 中的 request_id 注入到顶层字段
// （即使经过 WithGroup 嵌套，request_id 仍保持在日志根层级便于日志平台统一索引）。
type ContextHandler struct {
	base slog.Handler
	cur  slog.Handler
	ops  []handlerOp
}

func NewContextHandler(h slog.Handler) *ContextHandler {
	return &ContextHandler{
		base: h,
		cur:  h,
	}
}

func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.cur.Enabled(ctx, level)
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	requestID := RequestID(ctx)
	if requestID == "" {
		return h.cur.Handle(ctx, r)
	}
	if len(h.ops) == 0 {
		r = r.Clone()
		r.AddAttrs(slog.String("request_id", requestID))
		return h.base.Handle(ctx, r)
	}

	target := h.base.WithAttrs([]slog.Attr{slog.String("request_id", requestID)})
	for _, op := range h.ops {
		if op.group != "" {
			target = target.WithGroup(op.group)
		} else {
			target = target.WithAttrs(op.attrs)
		}
	}
	return target.Handle(ctx, r)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	nextOps := make([]handlerOp, len(h.ops)+1)
	copy(nextOps, h.ops)
	nextOps[len(h.ops)] = handlerOp{attrs: slices.Clone(attrs)}
	return &ContextHandler{
		base: h.base,
		cur:  h.cur.WithAttrs(attrs),
		ops:  nextOps,
	}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	nextOps := make([]handlerOp, len(h.ops)+1)
	copy(nextOps, h.ops)
	nextOps[len(h.ops)] = handlerOp{group: name}
	return &ContextHandler{
		base: h.base,
		cur:  h.cur.WithGroup(name),
		ops:  nextOps,
	}
}

// Setup 初始化全局默认 logger；format 默认为 FormatJSON，指定 FormatText 时使用易读文本格式。
func Setup(level slog.Level, format Format) {
	opts := &slog.HandlerOptions{Level: level}
	var baseHandler slog.Handler
	if format == FormatText {
		baseHandler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		baseHandler = slog.NewJSONHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(NewContextHandler(baseHandler)))
}
