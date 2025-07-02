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
	Code      int      `json:"code" example:"0"`                       // API业务状态码 (0=成功, 非0=失败)
	Message   string   `json:"message" example:"Success"`              // 响应消息
	Data      any      `json:"data"`                                   // 响应数据
	Errors    []string `json:"errors,omitempty"`                       // 错误详情列表
	Timestamp int64    `json:"timestamp" example:"1641024000"`         // 时间戳
	RequestID string   `json:"request_id,omitempty" example:"req_123"` // 请求ID
}

// ErrorResponse 错误响应结构 (HTTP状态码仍为200)
// 通过code字段(非0)区分具体的错误类型，参考pkg/errors/code.go中的错误码定义
type ErrorResponse struct {
	Code      int      `json:"code" example:"10001"`                   // 错误业务状态码 (非0值，具体含义见错误码表)
	Message   string   `json:"message" example:"请求参数错误"`               // 错误消息
	Data      any      `json:"data"`                                   // 响应数据 (错误时为null)
	Errors    []string `json:"errors,omitempty"`                       // 错误详情列表
	Timestamp int64    `json:"timestamp" example:"1641024000"`         // 时间戳
	RequestID string   `json:"request_id,omitempty" example:"req_123"` // 请求ID
}

// ResponseBuilder 响应构建器
type ResponseBuilder Response

// NewResponse 创建新的响应构建器
func NewResponse(c echo.Context) *ResponseBuilder {
	return &ResponseBuilder{
		Timestamp: time.Now().Unix(),
		Errors:    make([]string, 0),
		RequestID: context.MustGetRequestIDFromContext(c.Request().Context()),
	}
}

// WithCode 设置业务状态码
func (b *ResponseBuilder) WithCode(code int) *ResponseBuilder {
	b.Code = code
	return b
}

// WithMessage 设置响应消息
func (b *ResponseBuilder) WithMessage(message string) *ResponseBuilder {
	b.Message = message
	return b
}

// WithData 设置响应数据
func (b *ResponseBuilder) WithData(data any) *ResponseBuilder {
	b.Data = data
	return b
}

// WithErrors 设置错误详情列表
func (b *ResponseBuilder) WithErrors(errors []string) *ResponseBuilder {
	b.Errors = errors
	return b
}
