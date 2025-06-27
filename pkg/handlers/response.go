package handlers

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/context"
	"github.com/liukeshao/echo-template/pkg/errors"
)

// Response 统一的API响应结构
type Response struct {
	Code      string                `json:"code"`                 // API状态码 (ok=成功, 其他=失败)
	Message   string                `json:"message"`              // 响应消息
	Data      any                   `json:"data"`                 // 响应数据
	Errors    []*errors.ErrorDetail `json:"errors,omitempty"`     // 错误详情列表
	Timestamp int64                 `json:"timestamp"`            // 时间戳
	RequestID string                `json:"request_id,omitempty"` // 请求ID
}

// ResponseBuilder 响应构建器
type ResponseBuilder struct {
	response *Response
}

// NewResponse 创建新的响应构建器
func NewResponse() *ResponseBuilder {
	return &ResponseBuilder{
		response: &Response{
			Timestamp: time.Now().Unix(),
			Errors:    make([]*errors.ErrorDetail, 0),
		},
	}
}

// WithCode 设置业务状态码
func (b *ResponseBuilder) WithCode(code string) *ResponseBuilder {
	b.response.Code = code
	return b
}

// WithMessage 设置响应消息
func (b *ResponseBuilder) WithMessage(message string) *ResponseBuilder {
	b.response.Message = message
	return b
}

// WithData 设置响应数据
func (b *ResponseBuilder) WithData(data any) *ResponseBuilder {
	b.response.Data = data
	return b
}

// WithError 添加错误详情 (适配原有的field, message, code参数到errors.ErrorDetail)
func (b *ResponseBuilder) WithError(field, message, code string) *ResponseBuilder {
	error := &errors.ErrorDetail{
		Message:  message,
		Location: field, // 将field映射到Location
		Value:    code,  // 将code映射到Value
	}
	b.response.Errors = append(b.response.Errors, error)
	return b
}

// WithErrors 设置错误详情列表
func (b *ResponseBuilder) WithErrors(errors []*errors.ErrorDetail) *ResponseBuilder {
	b.response.Errors = errors
	return b
}

// WithRequestID 设置请求ID（可选，JSON方法会自动从context中获取）
func (b *ResponseBuilder) WithRequestID(requestID string) *ResponseBuilder {
	b.response.RequestID = requestID
	return b
}

// Build 构建响应
func (b *ResponseBuilder) Build() *Response {
	return b.response
}

// JSON 返回JSON响应，HTTP状态码统一为200
func (b *ResponseBuilder) JSON(c echo.Context) error {
	// 自动从context中获取request_id
	if requestID, ok := context.GetRequestIDFromContext(c.Request().Context()); ok {
		b.response.RequestID = requestID
	}

	return c.JSON(200, b.response)
}

// Success 成功响应
func Success(data any) *ResponseBuilder {
	return NewResponse().
		WithCode(errors.OK).
		WithMessage("Success").
		WithData(data)
}

// Error 错误响应
func Error(code string, message string) *ResponseBuilder {
	return NewResponse().
		WithCode(code).
		WithMessage(message).
		WithData(nil)
}

// ValidationError 验证错误响应
func ValidationError(message string, errorDetails []*errors.ErrorDetail) *ResponseBuilder {
	return NewResponse().
		WithCode(errors.UnprocessableEntity).
		WithMessage(message).
		WithData(nil).
		WithErrors(errorDetails)
}
