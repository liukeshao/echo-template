// Package status 实现 RPC 状态类型，对应 google.golang.org/grpc/status。
//
// Status 是不可变的 RPC 错误：code + message + details。
// 本包是纯抽象层，不含任何 HTTP、业务域或 reason 常量。
// detail payload 类型由 transport/rpc/errdetails 提供。
package status

import (
	"context"
	"errors"
	"fmt"

	"github.com/liukeshao/echo-template/transport/rpc/codes"
)

// Status 表示一个不可变的 RPC 状态，包含状态码、消息和可选的 detail payload 列表。
// 非 OK 的 Status 同时实现 error 接口。
// 应通过 New 创建，不应直接使用零值。
type Status struct {
	code    codes.Code
	message string
	details []any
}

// New 创建一个具有给定状态码和消息的 Status。
func New(c codes.Code, msg string) *Status {
	return &Status{code: c, message: msg}
}

// WithDetails 返回附加了 detail payload 的新 Status（原 Status 不可变）。
func (s *Status) WithDetails(details ...any) *Status {
	return &Status{
		code:    s.code,
		message: s.message,
		details: append(append([]any{}, s.details...), details...),
	}
}

// Code 返回 google.rpc.Code。
func (s *Status) Code() codes.Code {
	return s.code
}

// Message 返回面向开发者的调试消息（英文）。
func (s *Status) Message() string {
	return s.message
}

// Details 返回 detail payload 列表的只读副本。
func (s *Status) Details() []any {
	if len(s.details) == 0 {
		return nil
	}
	cp := make([]any, len(s.details))
	copy(cp, s.details)
	return cp
}

// Error 实现 error 接口。
func (s *Status) Error() string {
	return fmt.Sprintf("rpc error: code = %s desc = %s", s.code.Name(), s.message)
}

// As 判断 err 是否（或其链中）包含 *Status，等同于 errors.As。
func As(err error) (*Status, bool) {
	if s, ok := errors.AsType[*Status](err); ok {
		return s, true
	}
	return nil, false
}

// FromContextError 将 context 错误转换为对应的 Status。
func FromContextError(err error) *Status {
	if err == nil {
		return New(codes.OK, "")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return New(codes.DeadlineExceeded, err.Error())
	}
	if errors.Is(err, context.Canceled) {
		return New(codes.Canceled, err.Error())
	}
	return New(codes.Unknown, err.Error())
}
