package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/samber/oops"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/validator"
)

// ContentTypeProblemJSON 是 RFC 9457 Problem Details 的标准媒体类型。
const ContentTypeProblemJSON = "application/problem+json"

const statusClientClosedRequest = 499

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

// WriteProblem 将任意 error 分类并渲染为 RFC 9457 (application/problem+json) 响应。
//
// 分类优先级：
//  1. Problem / *Problem                  — 直接使用并补全缺省字段
//  2. oops.OopsError（含 Code）            — 经 pkg/codes 映射 HTTP 状态码并本地化
//  3. *validator.FieldViolations          — 400 VALIDATION_INVALID + 本地化字段 errors
//  4. *http.MaxBytesError                 — 413 REQUEST_ENTITY_TOO_LARGE
//  5. context.DeadlineExceeded / Canceled — 504 / 499
//  6. 其他未知错误                         — 500 INTERNAL_ERROR（隐藏内部细节）
func (rt *Runtime) WriteProblem(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	ctx := r.Context()
	loc := appctx.LocaleFromContext(ctx, rt.translator.Default())

	p := rt.classifyError(err, loc)
	if p.Type == "" {
		p.Type = "about:blank"
	}
	if p.Title == "" {
		p.Title = httpStatusTitle(p.Status)
	}
	if p.Instance == "" && r.URL != nil {
		p.Instance = r.URL.Path
	}
	if p.RequestID == "" {
		p.RequestID = appctx.RequestIDFromContext(ctx)
	}

	if p.Status >= http.StatusInternalServerError {
		slog.ErrorContext(ctx, "unhandled error", "status", p.Status, "code", p.Code, "error", err)
	}

	body, marshalErr := json.Marshal(p)
	if marshalErr != nil {
		slog.ErrorContext(ctx, "failed to marshal problem", "error", marshalErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", ContentTypeProblemJSON)
	w.WriteHeader(p.Status)
	_, _ = w.Write(body)
}

func (rt *Runtime) classifyError(err error, loc language.Tag) Problem {
	if p, ok := errors.AsType[Problem](err); ok {
		return rt.fillProblemDetail(p, loc)
	}
	if p, ok := errors.AsType[*Problem](err); ok && p != nil {
		return rt.fillProblemDetail(*p, loc)
	}
	if oopsErr, ok := errors.AsType[oops.OopsError](err); ok {
		if codeStr, _ := oopsErr.Code().(string); codeStr != "" {
			return rt.classifyOops(oopsErr, codeStr, loc)
		}
	}
	if fv, ok := errors.AsType[*validator.FieldViolations](err); ok {
		return rt.classifyFieldViolations(fv, loc)
	}
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return Problem{
			Status: http.StatusRequestEntityTooLarge,
			Code:   "REQUEST_ENTITY_TOO_LARGE",
			Detail: rt.translateFallback(loc, "fallback.request_entity_too_large", "Request body exceeds the maximum allowed size."),
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Problem{
			Status: http.StatusGatewayTimeout,
			Code:   "GATEWAY_TIMEOUT",
			Detail: rt.translateFallback(loc, "fallback.gateway_timeout", "The request timed out."),
		}
	}
	if errors.Is(err, context.Canceled) {
		return Problem{
			Status: statusClientClosedRequest,
			Title:  "Client Closed Request",
			Code:   "CLIENT_CLOSED_REQUEST",
			Detail: rt.translateFallback(loc, "fallback.client_closed_request", "Client closed the request before the server could respond."),
		}
	}
	return Problem{
		Status: http.StatusInternalServerError,
		Code:   "INTERNAL_ERROR",
		Detail: rt.translateFallbackInternal(loc),
	}
}

func (rt *Runtime) fillProblemDetail(p Problem, loc language.Tag) Problem {
	if p.Detail != "" {
		return p
	}
	if p.Code != "" {
		if msg, _, ok := rt.translator.T(loc, "fallback."+strings.ToLower(p.Code), nil); ok {
			p.Detail = msg
			return p
		}
	}
	p.Detail = httpStatusTitle(p.Status)
	return p
}

func (rt *Runtime) classifyOops(oopsErr oops.OopsError, codeStr string, loc language.Tag) Problem {
	status := codes.HTTPStatus(codes.Code(codeStr))
	params, _ := oopsErr.Context()[codes.CtxKeyParams].(map[string]string)

	detail, _, ok := rt.translator.T(loc, "errors."+codeStr, params)
	if !ok {
		if status >= http.StatusInternalServerError {
			detail = rt.translateFallbackInternal(loc)
		} else if pub := oops.GetPublic(oopsErr, ""); pub != "" {
			detail = pub
		} else {
			detail = httpStatusTitle(status)
		}
	}

	// 5xx 错误不对外透出 params，防止意外泄漏内部上下文。
	var publicParams map[string]string
	if status < http.StatusInternalServerError && len(params) > 0 {
		publicParams = params
	}
	return Problem{
		Status: status,
		Code:   codeStr,
		Detail: detail,
		Params: publicParams,
	}
}

func (rt *Runtime) classifyFieldViolations(fv *validator.FieldViolations, loc language.Tag) Problem {
	viols := fv.Items()
	fieldErrs := make([]FieldError, 0, len(viols))
	for _, it := range viols {
		msg, _, ok := rt.translator.T(loc, it.Key, it.Vars)
		if !ok {
			msg = it.Key
		}
		fieldErrs = append(fieldErrs, FieldError{
			Field:   it.Field,
			Message: msg,
		})
	}
	codeStr := codes.ValidationInvalid.String()
	detail, _, ok := rt.translator.T(loc, "errors."+codeStr, nil)
	if !ok {
		detail = "Request validation failed."
	}
	return Problem{
		Status: codes.HTTPStatus(codes.ValidationInvalid),
		Code:   codeStr,
		Detail: detail,
		Errors: fieldErrs,
	}
}

func (rt *Runtime) translateFallback(loc language.Tag, key, defaultMsg string) string {
	if msg, _, ok := rt.translator.T(loc, key, nil); ok {
		return msg
	}
	return defaultMsg
}

func (rt *Runtime) translateFallbackInternal(loc language.Tag) string {
	return rt.translateFallback(loc, "fallback.internal", "An internal server error occurred.")
}

func httpStatusTitle(status int) string {
	if status == statusClientClosedRequest {
		return "Client Closed Request"
	}
	if text := http.StatusText(status); text != "" {
		return text
	}
	return http.StatusText(http.StatusInternalServerError)
}
