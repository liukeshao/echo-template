package apperrs

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/appctx"
)

// EmptyData 空数据默认值
var EmptyData = map[string]any{}

// Response 统一的API响应结构
// HTTP状态码统一为200，通过code字段区分成功和失败，code=0表示成功，非0表示失败
type Response struct {
	Code      int            `json:"code"`                 // API业务状态码 (0=成功, 非0=失败)
	Message   string         `json:"message"`              // 响应消息
	Data      any            `json:"data"`                 // 响应数据
	Errors    []*ErrorDetail `json:"errors,omitempty"`     // 错误详情列表
	Timestamp int64          `json:"timestamp"`            // 时间戳
	RequestID string         `json:"request_id,omitempty"` // 请求ID
}

// Option 响应配置选项函数类型
type Option func(*Response)

// WithCode 设置业务状态码
func WithCode(code int) Option {
	return func(r *Response) {
		r.Code = code
	}
}

// WithMessage 设置响应消息
func WithMessage(message string) Option {
	return func(r *Response) {
		r.Message = message
	}
}

// WithData 设置响应数据
func WithData(data any) Option {
	return func(r *Response) {
		r.Data = data
	}
}

// WithErrors 设置错误详情列表
func WithErrors(errors []*ErrorDetail) Option {
	return func(r *Response) {
		r.Errors = errors
	}
}

// WithError 设置单个错误详情
func WithError(error *ErrorDetail) Option {
	return func(r *Response) {
		if r.Errors == nil {
			r.Errors = make([]*ErrorDetail, 0, 1)
		}
		r.Errors = append(r.Errors, error)
	}
}

// WithTimestamp 设置自定义时间戳
func WithTimestamp(timestamp int64) Option {
	return func(r *Response) {
		r.Timestamp = timestamp
	}
}

// WithRequestID 设置请求ID
func WithRequestID(requestID string) Option {
	return func(r *Response) {
		r.RequestID = requestID
	}
}

// NewResponse 创建响应（Options模式）
func NewResponse(c echo.Context, opts ...Option) *Response {
	// 初始化默认响应
	response := &Response{
		Code:      CodeOK.ToInt(),
		Message:   "success",
		Data:      EmptyData,
		Timestamp: time.Now().Unix(),
		Errors:    nil,
		RequestID: appctx.MustGetRequestIDFromContext(c.Request().Context()),
	}

	// 应用所有选项
	for _, opt := range opts {
		opt(response)
	}

	return response
}

// Error 实现error接口
func (r *Response) Error() string {
	if r.Code == 0 {
		return ""
	}

	if r.Message != "" {
		return r.Message
	}

	if len(r.Errors) > 0 {
		return r.Errors[0].Error()
	}

	return "unknown error"
}
