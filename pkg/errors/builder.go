package errors

import (
	"errors"
	"fmt"
)

// AppErrorBuilder 链式错误构建器
type AppErrorBuilder AppError

func newAppErrorBuilder() AppErrorBuilder {
	return AppErrorBuilder{
		code:    0,
		message: "",
		domain:  "",
		tags:    make([]string, 0),
		context: make(map[string]any),
		cause:   nil,
	}
}

// copy creates a deep copy of the current builder state.
func (b AppErrorBuilder) copy() AppErrorBuilder {
	return AppErrorBuilder{
		code:    b.code,
		message: b.message,
		domain:  b.domain,
		tags:    b.tags,
		context: b.context,
	}
}

// Code 设置错误码（链式方法）
func (b AppErrorBuilder) Code(code int) AppErrorBuilder {
	b2 := b.copy()
	b2.code = code
	return b2
}

func (b AppErrorBuilder) Message(message string) AppErrorBuilder {
	b2 := b.copy()
	b2.message = message
	return b2
}

// In sets the domain or feature category for the error.
func (b AppErrorBuilder) In(doamin string) AppErrorBuilder {
	b2 := b.copy()
	b2.domain = doamin
	return b2
}

// Tags 设置标签
func (b AppErrorBuilder) Tags(tags ...string) AppErrorBuilder {
	b2 := b.copy()
	b2.tags = append(b2.tags, tags...)
	return b
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

// Errorf 创建带格式化消息的错误
func (b AppErrorBuilder) Errorf(format string, args ...any) error {
	b2 := b.copy()
	b2.message = fmt.Sprintf(format, args...)
	return AppError(b2)
}

// Wrapf 包装错误并添加格式化消息
func (b AppErrorBuilder) Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}

	b2 := b.copy()
	b2.cause = err
	b2.message = fmt.Sprintf(format, args...)
	return AppError(b2)
}

func (b AppErrorBuilder) Wrap(err error) error {
	if err == nil {
		return nil
	}

	b2 := b.copy()
	b2.cause = err
	return AppError(b2)
}

func (b AppErrorBuilder) Join(err ...error) error {
	return b.Wrap(errors.Join(err...))
}

func (b AppErrorBuilder) New(message string) error {
	b2 := b.copy()
	b2.cause = errors.New(message)
	return AppError(b2)
}
