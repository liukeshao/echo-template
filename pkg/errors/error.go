package errors

import (
	"errors"
	"fmt"
	"log/slog"
)

type AppError struct {
	Code    string         // 错误码
	Message string         // 错误消息
	Context map[string]any // 结构化上下文
	Cause   error          // 原始错误
	In      string         // 服务名称
	Tags    []string       // 标签
	TraceID string         // 链路追踪ID
}

// With 添加结构化上下文字段（链式调用）
// 支持成对的键值参数：With("key1", "value1", "key2", "value2")
func (e AppError) With(keyvals ...any) AppError {
	if e.Context == nil {
		e.Context = make(map[string]any)
	}

	// 确保参数成对出现
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			if key, ok := keyvals[i].(string); ok {
				e.Context[key] = keyvals[i+1]
			}
		}
	}

	return e
}

// Wrap 包装原始错误，保留错误链
func (e AppError) Wrap(err error) AppError {
	if err == nil {
		return e
	}
	e.Cause = err
	return e
}

// Error 实现error接口
func (e AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 实现errors.Unwrap接口，支持errors.Is/errors.As
func (e AppError) Unwrap() error {
	return e.Cause
}

// LogValue 实现slog.LogValuer接口，支持结构化日志
func (e AppError) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("code", e.Code),
		slog.String("message", e.Message),
	}

	if e.In != "" {
		attrs = append(attrs, slog.String("in", e.In))
	}

	if len(e.Tags) > 0 {
		attrs = append(attrs, slog.Any("tags", e.Tags))
	}

	if e.TraceID != "" {
		attrs = append(attrs, slog.String("trace_id", e.TraceID))
	}

	// 添加结构化字段
	if e.Context != nil {
		for k, v := range e.Context {
			attrs = append(attrs, slog.Any(k, v))
		}
	}

	// 递归处理嵌套错误
	if e.Cause != nil {
		if appErr, ok := e.Cause.(AppError); ok {
			// 如果是AppError，递归调用LogValue
			attrs = append(attrs, slog.Any("cause", appErr.LogValue()))
		} else {
			// 普通错误直接记录
			attrs = append(attrs, slog.String("cause", e.Cause.Error()))
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
		return e.Code == appErr.Code
	}

	return errors.Is(e.Cause, target)
}

// As 实现errors.As接口
func (e AppError) As(target any) bool {
	if appErr, ok := target.(*AppError); ok {
		*appErr = e
		return true
	}

	if e.Cause != nil {
		return errors.As(e.Cause, target)
	}

	return false
}

// GetCode 获取错误码
func (e AppError) GetCode() string {
	return e.Code
}

// GetMessage 获取错误消息
func (e AppError) GetMessage() string {
	return e.Message
}

// GetContext 获取结构化字段
func (e AppError) GetContext() map[string]any {
	if e.Context == nil {
		return make(map[string]any)
	}
	// 返回副本，防止外部修改
	context := make(map[string]any)
	for k, v := range e.Context {
		context[k] = v
	}
	return context
}

// GetCause 获取原始错误
func (e AppError) GetCause() error {
	return e.Cause
}
