package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/samber/oops"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/pkg/appctx"
	appcodes "github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/i18n"
	"github.com/liukeshao/echo-template/pkg/validator"
	httpstatus "github.com/liukeshao/echo-template/transport/http/status"
	"github.com/liukeshao/echo-template/transport/rpc/codes"
	"github.com/liukeshao/echo-template/transport/rpc/errdetails"
	rpcstatus "github.com/liukeshao/echo-template/transport/rpc/status"
)

// registerErrorHandler 将 echo HTTPErrorHandler 绑定到使用给定 Translator 的实现上。
// Translator 以闭包方式注入，避免扩大 errorHandler 的参数面，同时保留对 DI 的依赖。
func registerErrorHandler(web *echo.Echo, t i18n.Translator) {
	web.HTTPErrorHandler = func(c *echo.Context, err error) {
		handleError(c, err, t)
	}
}

// handleError converts any error into an AIP-193 compliant JSON response.
//
// Conversion precedence:
//  1. *rpcstatus.Status         — use as-is, then inject ErrorInfo if absent
//  2. oops.OopsError with Code  — classifyOops (service/domain errors)
//  3. *validator.FieldViolations — field-level validation errors → BadRequest detail
//  4. context.DeadlineExceeded / context.Canceled
//  5. echo.HTTPStatusCoder      — map HTTP code → RPC code (framework errors)
//  6. everything else           — codes.Internal (message hidden for security)
func handleError(c *echo.Context, err error, t i18n.Translator) {
	if err == nil {
		return
	}

	ctx := c.Request().Context()
	loc := appctx.LocaleFromContext(ctx, t.Default())

	s := classify(err, t, loc)
	s = ensureErrorInfo(s)
	s = ensureLocalizedMessage(s, t, loc)
	s = ensureRequestInfo(s, ctx)

	if s.Code() == codes.Internal || s.Code() == codes.Unknown {
		slog.ErrorContext(ctx, "unhandled error", "error", err)
	}

	httpCode, resp := httpstatus.FromRPCStatus(s)

	if r, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil && r.Committed {
		return
	}
	if writeErr := c.JSON(httpCode, resp); writeErr != nil {
		slog.ErrorContext(ctx, "failed to write error response", "error", writeErr)
	}
}

// classify maps any error to a canonical *rpcstatus.Status.
func classify(err error, t i18n.Translator, loc language.Tag) *rpcstatus.Status {
	if s, ok := rpcstatus.As(err); ok {
		return s
	}
	if oopsErr, ok := errors.AsType[oops.OopsError](err); ok {
		if codeStr, _ := oopsErr.Code().(string); codeStr != "" {
			return classifyOops(oopsErr, codeStr)
		}
	}
	if fv, ok := errors.AsType[*validator.FieldViolations](err); ok {
		return classifyFieldViolations(fv, t, loc)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return rpcstatus.FromContextError(err)
	}
	// echo v5 has two error types: exported *HTTPError (NewHTTPError) and unexported
	// *httpError (ErrNotFound, ErrUnauthorized, etc.). Both implement HTTPStatusCoder,
	// so we match via the interface to cover both cases.
	// errors.AsType cannot be used here: HTTPStatusCoder does not implement error.
	var sc echo.HTTPStatusCoder
	if errors.As(err, &sc) {
		rpcCode := httpstatus.ToRPCCode(sc.StatusCode())
		var msg string
		if he, ok := sc.(*echo.HTTPError); ok && he.Message != "" {
			msg = he.Message
		} else {
			msg = http.StatusText(sc.StatusCode())
		}
		return rpcstatus.New(rpcCode, msg)
	}
	return rpcstatus.New(codes.Internal, "an internal server error occurred")
}

// classifyFieldViolations converts *validator.FieldViolations into an AIP-193 BadRequest Status.
//
// 每个 FieldViolation 的 description 由 Translator 基于 (key, vars) 渲染；
// 顶层 message 使用 VALIDATION_INVALID 的英文翻译作为开发者面向消息。
// LocalizedMessage 由 ensureLocalizedMessage 在调用方统一追加。
func classifyFieldViolations(fv *validator.FieldViolations, t i18n.Translator, loc language.Tag) *rpcstatus.Status {
	defTag := t.Default()

	viols := fv.Items()
	out := make([]errdetails.FieldViolation, 0, len(viols))
	for _, it := range viols {
		desc, _, ok := t.T(loc, it.Key, it.Vars)
		if !ok {
			desc = it.Key
		}
		out = append(out, errdetails.FieldViolation{
			Field:       it.Field,
			Description: desc,
		})
	}

	// 顶层 message 采用默认 locale 的 VALIDATION_INVALID，保持 error.message 为稳定英文。
	topMsg, _, ok := t.T(defTag, "errors."+appcodes.ValidationInvalid.String(), nil)
	if !ok {
		topMsg = "request validation failed"
	}

	return rpcstatus.New(codes.InvalidArgument, topMsg).WithDetails(
		errdetails.ErrorInfo{
			Type:   errdetails.TypeErrorInfo,
			Reason: appcodes.ValidationInvalid.String(),
			Domain: appcodes.Domain,
		},
		errdetails.BadRequest{
			Type:            errdetails.TypeBadRequest,
			FieldViolations: out,
		},
	)
}

// classifyOops converts an oops.OopsError with a registered app code into a
// *rpcstatus.Status, building AIP-193 details from the oops context:
//
//   - ErrorInfo.Reason    ← the app code string (e.g. "BOOK_NOT_FOUND")
//   - ErrorInfo.Metadata  ← oops context key CtxKeyMetadata (map[string]string)
//   - ResourceInfo        ← oops context key CtxKeyResourceInfo (errdetails.ResourceInfo)
//
// Message precedence for error.message (developer-facing English):
//  1. oops.Public() — set by the caller for a detailed, variable-rich description
//  2. oopsErr.Error() — fallback for non-internal errors
//  3. generic "an internal server error occurred" — always used for Internal/Unknown
//
// 本地化消息通过 ensureLocalizedMessage 附加到 details，不替换 error.message。
func classifyOops(oopsErr oops.OopsError, codeStr string) *rpcstatus.Status {
	rpcCode := appcodes.Lookup(appcodes.Code(codeStr))

	msg := oops.GetPublic(oopsErr, "")
	if msg == "" {
		if rpcCode == codes.Internal || rpcCode == codes.Unknown {
			msg = "an internal server error occurred"
		} else {
			msg = oopsErr.Error()
		}
	}

	errorInfo := errdetails.ErrorInfo{
		Type:   errdetails.TypeErrorInfo,
		Reason: codeStr,
		Domain: appcodes.Domain,
	}
	if metadata, ok := oopsErr.Context()[appcodes.CtxKeyMetadata].(map[string]string); ok {
		errorInfo.Metadata = metadata
	}

	details := []any{errorInfo}
	if ri, ok := oopsErr.Context()[appcodes.CtxKeyResourceInfo].(errdetails.ResourceInfo); ok {
		if ri.Type == "" {
			ri.Type = errdetails.TypeResourceInfo
		}
		details = append(details, ri)
	}

	return rpcstatus.New(rpcCode, msg).WithDetails(details...)
}

// ensureErrorInfo guarantees that the Status carries at least one ErrorInfo detail.
// AIP-193: "All error responses must include an ErrorInfo within details."
// Acts as a fallback for non-oops error paths (ent, context, echo.HTTPError).
func ensureErrorInfo(s *rpcstatus.Status) *rpcstatus.Status {
	for _, d := range s.Details() {
		if info, ok := d.(errdetails.ErrorInfo); ok && info.Type == errdetails.TypeErrorInfo {
			return s
		}
	}
	return s.WithDetails(errdetails.ErrorInfo{
		Type:   errdetails.TypeErrorInfo,
		Reason: s.Code().Name(),
		Domain: appcodes.Domain,
	})
}

// ensureLocalizedMessage 基于 Status 的 ErrorInfo（reason + metadata）渲染 LocalizedMessage。
//
// 查找顺序：
//  1. "errors.<reason>"（业务错误码或 ValidationInvalid）
//  2. "fallback.internal"（兜底消息）
//
// 若 Status 已包含 LocalizedMessage 则不追加。
func ensureLocalizedMessage(s *rpcstatus.Status, t i18n.Translator, loc language.Tag) *rpcstatus.Status {
	var (
		reason   string
		metadata map[string]string
	)
	for _, d := range s.Details() {
		if lm, ok := d.(errdetails.LocalizedMessage); ok && lm.Type == errdetails.TypeLocalizedMessage {
			return s
		}
		if info, ok := d.(errdetails.ErrorInfo); ok && info.Type == errdetails.TypeErrorInfo {
			reason = info.Reason
			metadata = info.Metadata
		}
	}
	if reason == "" {
		return s
	}

	text, used, ok := t.T(loc, "errors."+reason, metadata)
	if !ok {
		text, used, ok = t.T(loc, "fallback.internal", nil)
	}
	if !ok {
		return s
	}
	return s.WithDetails(errdetails.LocalizedMessage{
		Type:    errdetails.TypeLocalizedMessage,
		Locale:  used.String(),
		Message: text,
	})
}

// ensureRequestInfo 在上下文存在 request_id 时自动附加 AIP-193 RequestInfo，
// 便于客户端直接将错误响应体与服务端结构化日志关联排障。
func ensureRequestInfo(s *rpcstatus.Status, ctx context.Context) *rpcstatus.Status {
	rid, ok := appctx.GetRequestIDFromContext(ctx)
	if !ok || rid == "" {
		return s
	}
	for _, d := range s.Details() {
		if ri, ok := d.(errdetails.RequestInfo); ok && ri.Type == errdetails.TypeRequestInfo {
			return s
		}
	}
	return s.WithDetails(errdetails.RequestInfo{
		Type:      errdetails.TypeRequestInfo,
		RequestID: rid,
	})
}
