// Package httperr 提供零外部依赖的 RFC 9457 错误模型、有限 HTTP 状态码哨兵与可插拔错误编码器。
package httperr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/liukeshao/echo-template/pkg/validator"
)

const ContentTypeProblemJSON = "application/problem+json"

type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail"`
	Instance  string       `json:"instance,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
	Metadata  any          `json:"metadata,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

func (p Problem) Error() string {
	if p.Detail != "" {
		return fmt.Sprintf("http %d %s: %s", p.Status, StatusTitle(p.Status), p.Detail)
	}
	return fmt.Sprintf("http %d %s", p.Status, StatusTitle(p.Status))
}

type FieldError struct {
	Field    string            `json:"field"`
	Code     string            `json:"code"`
	Message  string            `json:"message"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Encoder func(w http.ResponseWriter, r *http.Request, err error)

// Write 补全缺省字段、同步标准响应头（Retry-After / WWW-Authenticate）并写出 RFC 9457 响应。
func Write(w http.ResponseWriter, r *http.Request, p Problem) {
	if p.Status == 0 {
		p.Status = http.StatusInternalServerError
	}
	if p.Status >= http.StatusInternalServerError {
		p.Metadata = nil
		p.Errors = nil
	}
	if ri, ok := p.Metadata.(RetryInfo); ok && ri.RetryAfter != "" && w.Header().Get("Retry-After") == "" {
		w.Header().Set("Retry-After", ri.RetryAfter)
	}
	if p.Status == http.StatusUnauthorized && w.Header().Get("WWW-Authenticate") == "" {
		if pi, ok := p.Metadata.(PreconditionInfo); ok && pi.Subject == "authorization" && pi.Detail != "" {
			w.Header().Set("WWW-Authenticate", pi.Detail)
		}
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

// Classify 将标准库、协议层与 httperr.Status 哨兵错误归一为原始 Problem（仅含 Status、Metadata、Errors，不含自然语言文案）。
// 显式 Status 哨兵优先于裸 context 超时/取消（确保下游客户端超时被显式标记为 503 时不被误改判为 504）；
// 请求自身的取消/超时由外层 Encoder 先检查 r.Context().Err() 处理。
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
				Code:     it.Code.Key(),
				Metadata: it.Metadata,
			})
		}
		return Problem{
			Status:   http.StatusBadRequest,
			Metadata: ValidationInfo{ErrorCount: len(viols)},
			Errors:   fieldErrs,
		}
	}
	if maxErr, ok := errors.AsType[*http.MaxBytesError](err); ok && maxErr != nil {
		return Problem{
			Status:   http.StatusRequestEntityTooLarge,
			Metadata: RequestLimitInfo{MaxBytes: maxErr.Limit},
		}
	}
	if s, ok := errors.AsType[Status](err); ok && s.Valid() {
		return Problem{
			Status: s.Int(),
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Problem{
			Status: http.StatusGatewayTimeout,
		}
	}
	if errors.Is(err, context.Canceled) {
		return Problem{
			Status: ClientClosedRequest.Int(),
		}
	}
	return Problem{
		Status: http.StatusInternalServerError,
	}
}

// StatusTitle 返回 HTTP 状态码对应的标准短标题（兼容 499 Client Closed Request）。
func StatusTitle(status int) string {
	if status == ClientClosedRequest.Int() {
		return "Client Closed Request"
	}
	if text := http.StatusText(status); text != "" {
		return text
	}
	return http.StatusText(http.StatusInternalServerError)
}
