package errors

import (
	"errors"
	"fmt"
	"log/slog"
)

type AppError struct {
	code    int            // 错误码
	message string         // 错误消息
	context map[string]any // 结构化上下文
	cause   error          // 原始错误
	domain  string         // 领域
	tags    []string       // 标签
}

// Error 实现error接口
func (e AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.code, e.message, e.cause)
	}
	return fmt.Sprintf("[%d] %s", e.code, e.message)
}

// Unwrap 实现errors.Unwrap接口，支持errors.Is/errors.As
func (e AppError) Unwrap() error {
	return e.cause
}

// LogValue 实现slog.LogValuer接口，支持结构化日志
func (e AppError) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.Int("code", e.code),
		slog.String("message", e.message),
	}

	if e.domain != "" {
		attrs = append(attrs, slog.String("domain", e.domain))
	}

	if len(e.tags) > 0 {
		attrs = append(attrs, slog.Any("tags", e.tags))
	}

	// 添加结构化字段
	if e.context != nil {
		for k, v := range e.context {
			attrs = append(attrs, slog.Any(k, v))
		}
	}

	// 递归处理嵌套错误
	if e.cause != nil {
		var appErr AppError
		if errors.As(e.cause, &appErr) {
			// 如果是AppError，递归调用LogValue
			attrs = append(attrs, slog.Any("cause", appErr.LogValue()))
		}
	}

	return slog.GroupValue(attrs...)
}

// Is 实现errors.Is接口
func (e AppError) Is(target error) bool {
	if target == nil {
		return false
	}

	var appErr AppError
	if errors.As(target, &appErr) {
		return e.code == appErr.code
	}

	return errors.Is(e.cause, target)
}

// As 实现errors.As接口
func (e AppError) As(target any) bool {
	if appErr, ok := target.(*AppError); ok {
		*appErr = e
		return true
	}

	if e.cause != nil {
		return errors.As(e.cause, target)
	}

	return false
}

// Code 获取错误码
func (e AppError) Code() int {
	return e.code
}

// Message 获取错误消息
func (e AppError) Message() string {
	return e.message
}

// Context 获取结构化字段
func (e AppError) Context() map[string]any {
	if e.context == nil {
		return make(map[string]any)
	}
	// 返回副本，防止外部修改
	context := make(map[string]any)
	for k, v := range e.context {
		context[k] = v
	}
	return context
}

// Cause 获取原始错误
func (e AppError) Cause() error {
	return e.cause
}
