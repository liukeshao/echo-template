package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/samber/oops"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/internal/codes"
	"github.com/liukeshao/echo-template/internal/i18n"
	"github.com/liukeshao/echo-template/pkg/logging"
	"github.com/liukeshao/echo-template/pkg/rest"
	"github.com/liukeshao/echo-template/pkg/validator"
)

// NewProblemEncoder 返回应用级 RFC 9457 ErrorEncoder，将错误统一分类并本地化：
//
// 分类优先级：
//  1. rest.Problem / *rest.Problem         — 优先按 errors.<CODE> / fallback.<code> 本地化
//  2. oops.OopsError（含 Code）             — 经 internal/codes 映射 HTTP 状态码并本地化
//  3. *validator.FieldViolations           — 400 VALIDATION_INVALID + 本地化字段 errors
//  4. *http.MaxBytesError                  — 413 REQUEST_ENTITY_TOO_LARGE
//  5. context.DeadlineExceeded / Canceled  — 504 / 499
//  6. 其他未知错误                          — 500 INTERNAL_ERROR（隐藏内部细节）
func NewProblemEncoder(tr i18n.Translator) rest.ErrorEncoder {
	if tr == nil {
		tr = noopTranslator{}
	}
	enc := &problemEncoder{translator: tr}
	return enc.Encode
}

type problemEncoder struct {
	translator i18n.Translator
}

func (e *problemEncoder) Encode(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	ctx := r.Context()
	loc := i18n.Locale(ctx, e.translator.Default())

	p := e.classifyError(err, loc)
	if p.RequestID == "" {
		p.RequestID = logging.RequestID(ctx)
	}
	if p.Status >= http.StatusInternalServerError {
		p.Params = nil
		slog.ErrorContext(ctx, "unhandled error", "status", p.Status, "code", p.Code, "error", err)
	}

	rest.WriteProblem(w, r, p)
}

func (e *problemEncoder) classifyError(err error, loc language.Tag) rest.Problem {
	if p, ok := errors.AsType[rest.Problem](err); ok {
		return e.localizeProblem(p, loc)
	}
	if p, ok := errors.AsType[*rest.Problem](err); ok && p != nil {
		return e.localizeProblem(*p, loc)
	}
	if oopsErr, ok := errors.AsType[oops.OopsError](err); ok {
		if codeStr, _ := oopsErr.Code().(string); codeStr != "" {
			return e.classifyOops(oopsErr, codeStr, loc)
		}
	}
	if fv, ok := errors.AsType[*validator.FieldViolations](err); ok {
		return e.classifyFieldViolations(fv, loc)
	}
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return rest.Problem{
			Status: http.StatusRequestEntityTooLarge,
			Code:   rest.CodeRequestEntityTooLarge,
			Detail: e.translateFallback(loc, "fallback.request_entity_too_large", "Request body exceeds the maximum allowed size."),
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return rest.Problem{
			Status: http.StatusGatewayTimeout,
			Code:   rest.CodeGatewayTimeout,
			Detail: e.translateFallback(loc, "fallback.gateway_timeout", "The request timed out."),
		}
	}
	if errors.Is(err, context.Canceled) {
		return rest.Problem{
			Status: rest.StatusClientClosedRequest,
			Title:  "Client Closed Request",
			Code:   rest.CodeClientClosedRequest,
			Detail: e.translateFallback(loc, "fallback.client_closed_request", "Client closed the request before the server could respond."),
		}
	}
	return rest.Problem{
		Status: http.StatusInternalServerError,
		Code:   rest.CodeInternalError,
		Detail: e.translateFallbackInternal(loc),
	}
}

func (e *problemEncoder) localizeProblem(p rest.Problem, loc language.Tag) rest.Problem {
	if p.Code != "" {
		if msg, _, ok := e.translator.T(loc, "errors."+p.Code, p.Params); ok {
			p.Detail = msg
			return p
		}
	}
	if p.Detail == "" && p.Code != "" {
		if msg, _, ok := e.translator.T(loc, "fallback."+strings.ToLower(p.Code), nil); ok {
			p.Detail = msg
			return p
		}
	}
	if p.Detail == "" {
		p.Detail = rest.StatusTitle(p.Status)
	}
	return p
}

func (e *problemEncoder) classifyOops(oopsErr oops.OopsError, codeStr string, loc language.Tag) rest.Problem {
	status := codes.HTTPStatus(codes.Code(codeStr))
	params, _ := oopsErr.Context()[codes.CtxKeyParams].(map[string]string)

	detail, _, ok := e.translator.T(loc, "errors."+codeStr, params)
	if !ok {
		if status >= http.StatusInternalServerError {
			detail = e.translateFallbackInternal(loc)
		} else if pub := oops.GetPublic(oopsErr, ""); pub != "" {
			detail = pub
		} else {
			detail = rest.StatusTitle(status)
		}
	}

	// 5xx 错误不对外透出 params，防止意外泄漏内部上下文。
	var publicParams map[string]string
	if status < http.StatusInternalServerError && len(params) > 0 {
		publicParams = params
	}
	return rest.Problem{
		Status: status,
		Code:   codeStr,
		Detail: detail,
		Params: publicParams,
	}
}

func (e *problemEncoder) classifyFieldViolations(fv *validator.FieldViolations, loc language.Tag) rest.Problem {
	viols := fv.Items()
	fieldErrs := make([]rest.FieldError, 0, len(viols))
	for _, it := range viols {
		msg, _, ok := e.translator.T(loc, it.Key, it.Vars)
		if !ok {
			msg = it.Key
		}
		fieldErrs = append(fieldErrs, rest.FieldError{
			Field:   it.Field,
			Message: msg,
		})
	}
	codeStr := codes.ValidationInvalid.String()
	detail, _, ok := e.translator.T(loc, "errors."+codeStr, nil)
	if !ok {
		detail = "Request validation failed."
	}
	return rest.Problem{
		Status: codes.HTTPStatus(codes.ValidationInvalid),
		Code:   codeStr,
		Detail: detail,
		Errors: fieldErrs,
	}
}

func (e *problemEncoder) translateFallback(loc language.Tag, key, defaultMsg string) string {
	if msg, _, ok := e.translator.T(loc, key, nil); ok {
		return msg
	}
	return defaultMsg
}

func (e *problemEncoder) translateFallbackInternal(loc language.Tag) string {
	return e.translateFallback(loc, "fallback.internal", "An internal server error occurred.")
}

type noopTranslator struct{}

func (noopTranslator) Match(string, string) language.Tag { return language.English }
func (noopTranslator) T(language.Tag, string, map[string]string) (string, language.Tag, bool) {
	return "", language.English, false
}
func (noopTranslator) Default() language.Tag { return language.English }
