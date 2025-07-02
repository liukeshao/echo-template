package log

import (
	"context"
	"log/slog"
	"os"

	"github.com/liukeshao/echo-template/config"
	appContext "github.com/liukeshao/echo-template/pkg/context"
)

type ContextHandler struct {
	slog.Handler
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	// 从 Context 中提取 Request ID
	requestID, ok := appContext.GetRequestIDFromContext(ctx)
	if ok {
		r.AddAttrs(slog.String("request_id", requestID))
	}
	return h.Handler.Handle(ctx, r)
}

// NewContextHandler 包装原始 Handler
func NewContextHandler(h slog.Handler) *ContextHandler {
	return &ContextHandler{Handler: h}
}

func Setup(cfg *config.Config) {
	// 原始 Handler（如 JSON Handler）
	baseHandler := slog.NewJSONHandler(os.Stdout, nil)

	// 使用自定义 ContextHandler 包装
	handler := NewContextHandler(baseHandler)

	// 设置为全局 Logger
	slog.SetDefault(slog.New(handler))

	// Configure logging.
	switch cfg.App.Environment {
	case config.EnvProduction:
		slog.SetLogLoggerLevel(slog.LevelInfo)
	default:
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
}
