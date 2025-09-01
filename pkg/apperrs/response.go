package apperrs

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/appctx"
)

var (
	EmptyData = map[string]any{}
)

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

// NewResponse 创建新的泛型响应
func NewResponse(c echo.Context) *Response {
	return initResponse(c)
}

func initResponse(c echo.Context) *Response {
	return &Response{
		Code:      OK,
		Message:   "success",
		Data:      EmptyData,
		Timestamp: time.Now().Unix(),
		Errors:    nil,
		RequestID: appctx.MustGetRequestIDFromContext(c.Request().Context()),
	}
}

// NewSuccessResponse 创建成功的响应
func NewSuccessResponse(c echo.Context, data any) *Response {
	response := initResponse(c)
	response.Code = 0
	response.Message = "success"
	if data != nil {
		response.Data = data
	}
	return response
}

// NewErrorResponse 创建失败的响应
func NewErrorResponse(c echo.Context, code int, message string) *Response {
	response := initResponse(c)
	response.Code = code
	response.Message = message
	return response
}

// WithCode 设置业务状态码
func (r *Response) WithCode(code int) *Response {
	r.Code = code
	return r
}

// WithMessage 设置响应消息
func (r *Response) WithMessage(message string) *Response {
	r.Message = message
	return r
}

// WithData 设置响应数据
func (r *Response) WithData(data any) *Response {
	r.Data = data
	return r
}

// WithErrors 设置错误详情列表
func (r *Response) WithErrors(errors []*ErrorDetail) *Response {
	r.Errors = errors
	return r
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
