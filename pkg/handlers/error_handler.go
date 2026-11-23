package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/samber/oops"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/i18n"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/validator"
)

const statusClientClosedRequest = 499

// registerErrorHandler 将 echo HTTPErrorHandler 绑定到基于 RFC 9457 的错误处理器。
func registerErrorHandler(web *echo.Echo, t i18n.Translator) {
	web.HTTPErrorHandler = func(c *echo.Context, err error) {
		handleError(c, err, t)
	}
}

// handleError 将任意 error 转换为 RFC 9457 (application/problem+json) 响应。
//
// 分类优先级：
//  1. types.Problem / *types.Problem — 直接使用并补全缺省字段
//  2. oops.OopsError（含 Code）       — 业务/领域错误，经 pkg/codes 映射 HTTP 状态码并本地化
//  3. *validator.FieldViolations     — 400 VALIDATION_INVALID + 本地化字段级 errors
//  4. context.DeadlineExceeded / Canceled — 504 / 499
//  5. echo.HTTPStatusCoder           — 框架错误（404 / 405 / 413 等），保留原 HTTP 状态码
//  6. 其他未知错误                    — 500 INTERNAL_ERROR（隐藏内部细节）
func handleError(c *echo.Context, err error, t i18n.Translator) {
	if err == nil {
		return
	}

	ctx := c.Request().Context()
	loc := appctx.LocaleFromContext(ctx, t.Default())

	p := classifyError(err, t, loc)
	if p.Type == "" {
		p.Type = "about:blank"
	}
	if p.Title == "" {
		p.Title = httpStatusTitle(p.Status)
	}
	if p.Instance == "" && c.Request() != nil && c.Request().URL != nil {
		p.Instance = c.Request().URL.Path
	}
	if p.RequestID == "" {
		p.RequestID = appctx.MustGetRequestIDFromContext(ctx)
	}

	if p.Status >= http.StatusInternalServerError {
		slog.ErrorContext(ctx, "unhandled error", "status", p.Status, "code", p.Code, "error", err)
	}

	if r, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil && r.Committed {
		return
	}

	body, marshalErr := json.Marshal(p)
	if marshalErr != nil {
		slog.ErrorContext(ctx, "failed to marshal problem response", "error", marshalErr)
		return
	}
	if writeErr := c.Blob(p.Status, types.ContentTypeProblemJSON, body); writeErr != nil {
		slog.ErrorContext(ctx, "failed to write problem response", "error", writeErr)
	}
}

func classifyError(err error, t i18n.Translator, loc language.Tag) types.Problem {
	if p, ok := errors.AsType[types.Problem](err); ok {
		return p
	}
	if p, ok := errors.AsType[*types.Problem](err); ok && p != nil {
		return *p
	}
	if oopsErr, ok := errors.AsType[oops.OopsError](err); ok {
		if codeStr, _ := oopsErr.Code().(string); codeStr != "" {
			return classifyOops(oopsErr, codeStr, t, loc)
		}
	}
	if fv, ok := errors.AsType[*validator.FieldViolations](err); ok {
		return classifyFieldViolations(fv, t, loc)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return types.Problem{
			Status: http.StatusGatewayTimeout,
			Code:   httpStatusCode(http.StatusGatewayTimeout),
			Detail: http.StatusText(http.StatusGatewayTimeout),
		}
	}
	if errors.Is(err, context.Canceled) {
		return types.Problem{
			Status: statusClientClosedRequest,
			Title:  "Client Closed Request",
			Code:   "CLIENT_CLOSED_REQUEST",
			Detail: "Client closed the request before the server could respond.",
		}
	}
	// echo v5 同时存在导出的 *HTTPError 与未导出的 *httpError（如 ErrNotFound），
	// 二者均实现 HTTPStatusCoder 接口。
	var sc echo.HTTPStatusCoder
	if errors.As(err, &sc) {
		status := sc.StatusCode()
		detail := httpStatusTitle(status)
		if status >= http.StatusInternalServerError {
			detail = translateFallbackInternal(t, loc)
		} else if he, ok := sc.(*echo.HTTPError); ok && he.Message != "" {
			detail = he.Message
		}
		return types.Problem{
			Status: status,
			Code:   httpStatusCode(status),
			Detail: detail,
		}
	}

	return types.Problem{
		Status: http.StatusInternalServerError,
		Code:   "INTERNAL_ERROR",
		Detail: translateFallbackInternal(t, loc),
	}
}

func classifyOops(oopsErr oops.OopsError, codeStr string, t i18n.Translator, loc language.Tag) types.Problem {
	status := codes.HTTPStatus(codes.Code(codeStr))
	params, _ := oopsErr.Context()[codes.CtxKeyParams].(map[string]string)

	detail, _, ok := t.T(loc, "errors."+codeStr, params)
	if !ok {
		if status >= http.StatusInternalServerError {
			detail = translateFallbackInternal(t, loc)
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

	return types.Problem{
		Status: status,
		Code:   codeStr,
		Detail: detail,
		Params: publicParams,
	}
}

func classifyFieldViolations(fv *validator.FieldViolations, t i18n.Translator, loc language.Tag) types.Problem {
	viols := fv.Items()
	fieldErrs := make([]types.FieldError, 0, len(viols))
	for _, it := range viols {
		msg, _, ok := t.T(loc, it.Key, it.Vars)
		if !ok {
			msg = it.Key
		}
		fieldErrs = append(fieldErrs, types.FieldError{
			Field:   it.Field,
			Message: msg,
		})
	}

	codeStr := codes.ValidationInvalid.String()
	detail, _, ok := t.T(loc, "errors."+codeStr, nil)
	if !ok {
		detail = "Request validation failed."
	}

	return types.Problem{
		Status: codes.HTTPStatus(codes.ValidationInvalid),
		Code:   codeStr,
		Detail: detail,
		Errors: fieldErrs,
	}
}

func translateFallbackInternal(t i18n.Translator, loc language.Tag) string {
	if msg, _, ok := t.T(loc, "fallback.internal", nil); ok {
		return msg
	}
	return "An internal server error occurred."
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

func httpStatusCode(status int) string {
	if status == http.StatusInternalServerError {
		return "INTERNAL_ERROR"
	}
	title := httpStatusTitle(status)
	if title == "" {
		return fmt.Sprintf("HTTP_%d", status)
	}
	return strings.ToUpper(strings.NewReplacer(" ", "_", "-", "_").Replace(title))
}
