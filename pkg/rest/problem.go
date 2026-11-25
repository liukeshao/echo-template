// Package rest 提供基于 Go 1.22+ 标准库 net/http 的轻量、零外部依赖 REST 协议原语，
// 配合 TypeSpec Emitter 生成的 api.NewXxxHandler 实现 Connect-Go v2 风格的零反射传输层。
package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/liukeshao/echo-template/pkg/validator"
)

// ContentTypeProblemJSON 是 RFC 9457 Problem Details 的标准媒体类型。
const ContentTypeProblemJSON = "application/problem+json"

// HTTP 传输层与协议通用错误码常量。
const (
	CodeBadRequest                = "BAD_REQUEST"
	CodeNotFound                  = "NOT_FOUND"
	CodeMethodNotAllowed          = "METHOD_NOT_ALLOWED"
	CodeRequestEntityTooLarge     = "REQUEST_ENTITY_TOO_LARGE"
	CodeGatewayTimeout            = "GATEWAY_TIMEOUT"
	CodeClientClosedRequest       = "CLIENT_CLOSED_REQUEST"
	CodeInternalError             = "INTERNAL_ERROR"
	CodeValidationInvalid         = "VALIDATION_INVALID"
	CodePatchInvalid              = "PATCH_INVALID"
	CodePatchUnsupportedMediaType = "PATCH_UNSUPPORTED_MEDIA_TYPE"
)

// StatusClientClosedRequest 是客户端提前断开连接时使用的非标准 HTTP 状态码（499）。
const StatusClientClosedRequest = 499

// Problem 遵循 RFC 9457 (Problem Details for HTTP APIs) 的扁平、强类型 HTTP 错误响应。
type Problem struct {
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	Status    int               `json:"status"`
	Detail    string            `json:"detail"`
	Instance  string            `json:"instance,omitempty"`
	Code      string            `json:"code"`
	RequestID string            `json:"request_id,omitempty"`
	Params    map[string]string `json:"params,omitempty"`
	Errors    []FieldError      `json:"errors,omitempty"`
}

// Error 实现 error 接口，便于直接作为 error 返回或供 HTTP 客户端消费。
func (p Problem) Error() string {
	return fmt.Sprintf("http %d %s: %s", p.Status, p.Code, p.Detail)
}

// FieldError 描述单个请求字段的校验失败原因。
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ErrorEncoder 将任意 error 渲染为 HTTP 错误响应（通常为 RFC 9457 application/problem+json）。
// 应用层可注入自定义 ErrorEncoder 以接入业务错误码注册表、i18n 与链路追踪 ID。
type ErrorEncoder func(w http.ResponseWriter, r *http.Request, err error)

// WriteProblem 补全 Problem 缺省字段并写出 RFC 9457 (application/problem+json) 响应。
func WriteProblem(w http.ResponseWriter, r *http.Request, p Problem) {
	if p.Status == 0 {
		p.Status = http.StatusInternalServerError
	}
	if p.Type == "" {
		p.Type = "about:blank"
	}
	if p.Title == "" {
		p.Title = StatusTitle(p.Status)
	}
	if p.Detail == "" {
		p.Detail = p.Title
	}
	if p.Instance == "" && r != nil && r.URL != nil {
		p.Instance = r.URL.Path
	}

	body, err := json.Marshal(p)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to marshal problem", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", ContentTypeProblemJSON)
	w.WriteHeader(p.Status)
	_, _ = w.Write(body)
}

// DefaultErrorEncoder 是不依赖外部业务错误库或 i18n 的默认 RFC 9457 编码器，
// 适合独立单测或轻量服务零配置运行。
func DefaultErrorEncoder(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	p := classifyDefaultError(err)
	if p.Status >= http.StatusInternalServerError {
		p.Params = nil
		slog.ErrorContext(r.Context(), "unhandled error", "status", p.Status, "code", p.Code, "error", err)
	}
	WriteProblem(w, r, p)
}

func classifyDefaultError(err error) Problem {
	if p, ok := errors.AsType[Problem](err); ok {
		return p
	}
	if p, ok := errors.AsType[*Problem](err); ok && p != nil {
		return *p
	}
	if fv, ok := errors.AsType[*validator.FieldViolations](err); ok {
		viols := fv.Items()
		fieldErrs := make([]FieldError, 0, len(viols))
		for _, it := range viols {
			fieldErrs = append(fieldErrs, FieldError{
				Field:   it.Field,
				Message: it.Key,
			})
		}
		return Problem{
			Status: http.StatusBadRequest,
			Code:   CodeValidationInvalid,
			Detail: "Request validation failed.",
			Errors: fieldErrs,
		}
	}
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return Problem{
			Status: http.StatusRequestEntityTooLarge,
			Code:   CodeRequestEntityTooLarge,
			Detail: "Request body exceeds the maximum allowed size.",
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Problem{
			Status: http.StatusGatewayTimeout,
			Code:   CodeGatewayTimeout,
			Detail: "The request timed out.",
		}
	}
	if errors.Is(err, context.Canceled) {
		return Problem{
			Status: StatusClientClosedRequest,
			Title:  "Client Closed Request",
			Code:   CodeClientClosedRequest,
			Detail: "Client closed the request before the server could respond.",
		}
	}
	return Problem{
		Status: http.StatusInternalServerError,
		Code:   CodeInternalError,
		Detail: "An internal server error occurred.",
	}
}

// StatusTitle 返回 HTTP 状态码对应的标准短标题（兼容 499 Client Closed Request）。
func StatusTitle(status int) string {
	if status == StatusClientClosedRequest {
		return "Client Closed Request"
	}
	if text := http.StatusText(status); text != "" {
		return text
	}
	return http.StatusText(http.StatusInternalServerError)
}
