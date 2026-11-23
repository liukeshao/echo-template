package types

import "fmt"

// ContentTypeProblemJSON 是 RFC 9457 Problem Details 的标准媒体类型。
const ContentTypeProblemJSON = "application/problem+json"

// Problem 遵循 RFC 9457 (Problem Details for HTTP APIs) 的扁平、强类型 HTTP 错误响应。
//
// 标准成员（type / title / status / detail / instance）加常用扩展成员（code / request_id / params / errors）：
//
//	{
//	  "type": "about:blank",
//	  "title": "Bad Request",
//	  "status": 400,
//	  "detail": "请求参数校验失败。",
//	  "instance": "/api/v1/books",
//	  "code": "VALIDATION_INVALID",
//	  "request_id": "01J...",
//	  "errors": [
//	    { "field": "title", "message": "title 不能为空。" }
//	  ]
//	}
type Problem struct {
	// Type 标识问题类型的 URI 引用；缺省使用 "about:blank"。
	Type string `json:"type" example:"about:blank"`
	// Title 与 HTTP 状态码对应的简短英文摘要（不随单次错误实例变化）。
	Title string `json:"title" example:"Not Found"`
	// Status HTTP 状态码，例如 400 / 404 / 409 / 500。
	Status int `json:"status" example:"404"`
	// Detail 面向本次请求的具体错误说明（已按 Accept-Language 本地化）。
	Detail string `json:"detail" example:"Book 01ARZ3NDEKTSV4RRFFQ69G5FAV was not found."`
	// Instance 触发本次错误的请求路径，例如 "/api/v1/books/01ARZ3NDEKTSV4RRFFQ69G5FAV"。
	Instance string `json:"instance,omitempty" example:"/api/v1/books/01ARZ3NDEKTSV4RRFFQ69G5FAV"`
	// Code 机器可读的业务错误码（UPPER_SNAKE_CASE），例如 "BOOK_NOT_FOUND"。
	Code string `json:"code" example:"BOOK_NOT_FOUND"`
	// RequestID 与响应头 X-Request-ID 及服务端结构化日志一致的追踪 ID。
	RequestID string `json:"request_id,omitempty" example:"01ARZ3NDEKTSV4RRFFQ69G5FAV"`
	// Params 与本次错误相关的结构化上下文参数（例如 book_id / email）。
	Params map[string]string `json:"params,omitempty"`
	// Errors 字段级校验失败明细，仅在请求参数校验不通过时返回。
	Errors []FieldError `json:"errors,omitempty"`
}

// Error 实现 error 接口，方便 HTTP 客户端在反序列化后直接作为 error 返回。
func (p Problem) Error() string {
	return fmt.Sprintf("http %d %s: %s", p.Status, p.Code, p.Detail)
}

// FieldError 描述单个请求字段的校验失败原因。
type FieldError struct {
	Field   string `json:"field" example:"title"`
	Message string `json:"message" example:"title is required."`
}
