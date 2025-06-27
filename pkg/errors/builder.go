package errors

import (
	"fmt"
)

// AppErrorBuilder 链式错误构建器
type AppErrorBuilder struct {
	code    string
	message string
	service string
	tags    []string
	traceID string
	context map[string]any
	cause   error
}

func newAppErrorBuilder() AppErrorBuilder {
	return AppErrorBuilder{
		code:    "",
		message: "",
		service: "",
		tags:    make([]string, 0),
		traceID: "",
		context: make(map[string]any),
		cause:   nil,
	}
}

// copy creates a deep copy of the current builder state.
func (b AppErrorBuilder) copy() AppErrorBuilder {
	return AppErrorBuilder{
		code:    b.code,
		message: b.message,
		service: b.service,
		tags:    b.tags,
		traceID: b.traceID,
		context: b.context,
	}
}

// Code 设置错误码（链式方法）
func (b AppErrorBuilder) Code(code string) AppErrorBuilder {
	b2 := b.copy()
	b2.code = code
	return b2
}

func (b AppErrorBuilder) Message(message string) AppErrorBuilder {
	b2 := b.copy()
	b2.message = message
	return b2
}

// In 设置服务名称
func (b AppErrorBuilder) In(service string) AppErrorBuilder {
	b2 := b.copy()
	b2.service = service
	return b2
}

// Tags 设置标签
func (b AppErrorBuilder) Tags(tags ...string) AppErrorBuilder {
	b2 := b.copy()
	b2.tags = append(b2.tags, tags...)
	return b
}

// Trace 设置链路追踪ID
func (b AppErrorBuilder) Trace(traceID string) AppErrorBuilder {
	b2 := b.copy()
	b2.traceID = traceID
	return b2
}

// With 添加上下文信息
func (b AppErrorBuilder) With(key string, value any) AppErrorBuilder {
	b2 := b.copy()
	if b2.context == nil {
		b2.context = make(map[string]any)
	}
	b2.context[key] = value
	return b2
}

// build 构建 AppError
func (b AppErrorBuilder) build() AppError {
	return AppError{
		Code:    b.code,
		Message: "",
		Service: b.service,
		Tags:    b.tags,
		TraceID: b.traceID,
		Context: b.context,
		Cause:   b.cause,
	}
}

// Errorf 创建带格式化消息的错误
func (b AppErrorBuilder) Errorf(format string, args ...any) error {
	b2 := b.copy()
	b2.message = fmt.Sprintf(format, args...)
	return b2.build()
}

// Wrapf 包装错误并添加格式化消息
func (b AppErrorBuilder) Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}

	b2 := b.copy()
	b2.cause = err
	b2.message = fmt.Sprintf(format, args...)
	return b2.build()
}

func (b AppErrorBuilder) Wrap(err error) error {
	if err == nil {
		return nil
	}

	b2 := b.copy()
	b2.cause = err
	return b2.build()
}
