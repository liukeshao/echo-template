package handlers

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/context"
)

// Response 统一的API响应结构
// HTTP状态码统一为200，通过code字段区分成功和失败
// code=0表示成功，非0表示失败，具体错误码含义请参考pkg/errors/code.go
type Response struct {
	Code      int      `json:"code"`                 // API业务状态码 (0=成功, 非0=失败)
	Message   string   `json:"message"`              // 响应消息
	Data      any      `json:"data"`                 // 响应数据
	Errors    []string `json:"errors,omitempty"`     // 错误详情列表
	Timestamp int64    `json:"timestamp"`            // 时间戳
	RequestID string   `json:"request_id,omitempty"` // 请求ID
}

// NewResponse 创建新的泛型响应
func NewResponse(c echo.Context) *Response {
	return &Response{
		Timestamp: time.Now().Unix(),
		Errors:    make([]string, 0),
		RequestID: context.MustGetRequestIDFromContext(c.Request().Context()),
	}
}

// WithCode 设置业务状态码
func (b *Response) WithCode(code int) *Response {
	b.Code = code
	return b
}

// WithMessage 设置响应消息
func (b *Response) WithMessage(message string) *Response {
	b.Message = message
	return b
}

// WithData 设置响应数据
func (b *Response) WithData(data any) *Response {
	b.Data = data
	return b
}

// WithErrors 设置错误详情列表
func (b *Response) WithErrors(errors []string) *Response {
	b.Errors = errors
	return b
}
