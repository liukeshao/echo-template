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

// Problem 遵循 RFC 9457 (Problem Details for HTTP APIs) 的扁平、强类型 HTTP 错误响应，
// 结合有限标准 HTTP 错误码（Code）、AIP-193 风格结构化 Metadata 与字段级 FieldError。
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail"`
	Instance  string       `json:"instance,omitempty"`
	Code      Code         `json:"code"`
	RequestID string       `json:"request_id,omitempty"`
	Metadata  any          `json:"metadata,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

// Error 实现 error 接口，便于作为协议错误传递或供 HTTP 客户端消费。
func (p Problem) Error() string {
	if p.Detail != "" {
		return fmt.Sprintf("http %d %s: %s", p.Status, p.Code, p.Detail)
	}
	return fmt.Sprintf("http %d %s", p.Status, p.Code)
}

// FieldError 描述单个请求字段的校验失败原因、规则码及约束元数据。
type FieldError struct {
	Field    string            `json:"field"`
	Code     string            `json:"code"`
	Message  string            `json:"message"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ErrorEncoder 将任意 error 渲染为 HTTP 错误响应（通常为 RFC 9457 application/problem+json）。
// 应用层可注入自定义 ErrorEncoder 以接入 oops 诊断链、统一 i18n 与链路追踪 ID。
type ErrorEncoder func(w http.ResponseWriter, r *http.Request, err error)

// WriteProblem 补全 Problem 缺省字段、同步标准协议响应头（如 Retry-After）并写出 RFC 9457 响应。
func WriteProblem(w http.ResponseWriter, r *http.Request, p Problem) {
	if p.Status == 0 {
		if p.Code != "" {
			p.Status = p.Code.HTTPStatus()
		} else {
			p.Status = http.StatusInternalServerError
		}
	}
	if p.Status >= http.StatusInternalServerError {
		// 纵深防御：5xx 错误绝不对外输出任何元数据或字段明细。
		p.Metadata = nil
		p.Errors = nil
	}
	if ri, ok := p.Metadata.(RetryInfo); ok && ri.RetryAfter != "" && w.Header().Get("Retry-After") == "" {
		w.Header().Set("Retry-After", ri.RetryAfter)
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
		ctx := context.Background()
		if r != nil {
			ctx = r.Context()
		}
		slog.ErrorContext(ctx, "failed to marshal problem", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", ContentTypeProblemJSON)
	w.WriteHeader(p.Status)
	_, _ = w.Write(body)
}

// Classify 将标准库与传输协议层错误归一为原始 Problem（仅含 Status、Code、Metadata、Errors，不含自然语言文案）。
// 供 DefaultErrorEncoder 与应用层自定义 ErrorEncoder（如 server.NewProblemEncoder）共同复用，消除重复分类逻辑。
func Classify(err error) Problem {
	if p, ok := errors.AsType[Problem](err); ok {
		return p
	}
	if p, ok := errors.AsType[*Problem](err); ok && p != nil {
		return *p
	}
	if fv, ok := errors.AsType[*validator.FieldViolations](err); ok && fv != nil {
		viols := fv.Items()
		fieldErrs := make([]FieldError, 0, len(viols))
		for _, it := range viols {
			fieldErrs = append(fieldErrs, FieldError{
				Field:    it.Field,
				Code:     it.Code,
				Metadata: it.Metadata,
			})
		}
		return Problem{
			Status:   CodeBadRequest.HTTPStatus(),
			Code:     CodeBadRequest,
			Metadata: ValidationInfo{ErrorCount: len(viols)},
			Errors:   fieldErrs,
		}
	}
	if maxErr, ok := errors.AsType[*http.MaxBytesError](err); ok && maxErr != nil {
		return Problem{
			Status:   CodeContentTooLarge.HTTPStatus(),
			Code:     CodeContentTooLarge,
			Metadata: RequestLimitInfo{MaxBytes: maxErr.Limit},
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Problem{
			Status: CodeGatewayTimeout.HTTPStatus(),
			Code:   CodeGatewayTimeout,
		}
	}
	if errors.Is(err, context.Canceled) {
		return Problem{
			Status: CodeClientClosedRequest.HTTPStatus(),
			Title:  "Client Closed Request",
			Code:   CodeClientClosedRequest,
		}
	}
	if c, ok := errors.AsType[Code](err); ok && c != "" {
		return Problem{
			Status: c.HTTPStatus(),
			Code:   c,
		}
	}
	return Problem{
		Status: CodeInternal.HTTPStatus(),
		Code:   CodeInternal,
	}
}

// DefaultErrorEncoder 是不依赖外部业务错误库或 i18n 的默认 RFC 9457 编码器，
// 适合独立单测或轻量服务零配置运行。
func DefaultErrorEncoder(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	p := Classify(err)
	for i := range p.Errors {
		if p.Errors[i].Message == "" {
			p.Errors[i].Message = p.Errors[i].Code
		}
	}
	if p.Status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "unhandled error", "status", p.Status, "code", p.Code, "error", err)
	}
	WriteProblem(w, r, p)
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
