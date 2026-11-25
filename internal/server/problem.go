package server

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"net/http"

	"github.com/samber/oops"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/internal/codes"
	"github.com/liukeshao/echo-template/internal/i18n"
	"github.com/liukeshao/echo-template/pkg/logging"
	"github.com/liukeshao/echo-template/pkg/rest"
	"github.com/liukeshao/echo-template/pkg/validator"
)

// NewProblemEncoder 返回应用级 RFC 9457 ErrorEncoder，按三段式流水线处理所有错误：
//  1. Classify：纯提取 Status + Code + 强类型 Metadata + FieldErrors（不生成自然语言文案，不读取 oops.Public）
//  2. Sanitize & Log：注入 RequestID；5xx 记录完整结构化错误链（含 oops stack/context/user）并清空对外 Metadata
//  3. Unified Localize：唯一国际化出口，按请求 Locale 统一渲染 errors.<CODE> 与 validation.<CODE>
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

	// Stage 1: 纯分类与强类型 Metadata 提取
	p := classifyError(err)

	// Stage 2: 安全隔离与结构化日志
	if p.RequestID == "" {
		p.RequestID = logging.RequestID(ctx)
	}
	if p.Status >= http.StatusInternalServerError {
		p.Metadata = nil
		slog.ErrorContext(ctx, "unhandled error", "status", p.Status, "code", p.Code, "error", err)
	}

	// Stage 3: 终端统一国际化
	e.localizeProblem(&p, loc)

	rest.WriteProblem(w, r, p)
}

func classifyError(err error) rest.Problem {
	if p, ok := errors.AsType[rest.Problem](err); ok {
		return p
	}
	if p, ok := errors.AsType[*rest.Problem](err); ok && p != nil {
		return *p
	}
	if oopsErr, ok := oops.AsOops(err); ok {
		if code, ok := codes.FromAny(oopsErr.Code()); ok {
			status := codes.HTTPStatus(code)
			var meta rest.Metadata
			if status < http.StatusInternalServerError {
				if m, ok := oopsErr.Context()[rest.MetaKey].(rest.Metadata); ok {
					meta = m
				}
			}
			return rest.Problem{
				Status:   status,
				Code:     code.String(),
				Metadata: meta,
			}
		}
	}
	if fv, ok := errors.AsType[*validator.FieldViolations](err); ok && fv != nil {
		viols := fv.Items()
		fieldErrs := make([]rest.FieldError, 0, len(viols))
		for _, it := range viols {
			fieldErrs = append(fieldErrs, rest.FieldError{
				Field:    it.Field,
				Code:     it.Code,
				Metadata: it.Metadata,
			})
		}
		return rest.Problem{
			Status:   codes.HTTPStatus(codes.ValidationInvalid),
			Code:     codes.ValidationInvalid.String(),
			Metadata: rest.ValidationInfo{ErrorCount: len(viols)},
			Errors:   fieldErrs,
		}
	}
	if maxErr, ok := errors.AsType[*http.MaxBytesError](err); ok && maxErr != nil {
		return rest.Problem{
			Status:   http.StatusRequestEntityTooLarge,
			Code:     rest.CodeRequestEntityTooLarge,
			Metadata: rest.RequestLimitInfo{MaxBytes: maxErr.Limit},
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return rest.Problem{
			Status: http.StatusGatewayTimeout,
			Code:   rest.CodeGatewayTimeout,
		}
	}
	if errors.Is(err, context.Canceled) {
		return rest.Problem{
			Status: rest.StatusClientClosedRequest,
			Title:  "Client Closed Request",
			Code:   rest.CodeClientClosedRequest,
		}
	}
	return rest.Problem{
		Status: http.StatusInternalServerError,
		Code:   rest.CodeInternalError,
	}
}

func (e *problemEncoder) localizeProblem(p *rest.Problem, loc language.Tag) {
	if p.Code != "" {
		vars := metadataI18nVars(p.Metadata)
		if msg, _, ok := e.translator.T(loc, "errors."+p.Code, vars); ok {
			p.Detail = msg
		}
	}
	if p.Detail == "" {
		p.Detail = rest.StatusTitle(p.Status)
	}

	for i := range p.Errors {
		fe := &p.Errors[i]
		if fe.Code == "" {
			continue
		}
		vars := withFieldVar(fe.Field, fe.Metadata)
		if msg, _, ok := e.translator.T(loc, "validation."+fe.Code, vars); ok {
			fe.Message = msg
		} else if fe.Message == "" {
			fe.Message = fe.Code
		}
	}
}

func metadataI18nVars(meta any) map[string]string {
	switch m := meta.(type) {
	case rest.Metadata:
		return m.I18nVars()
	case map[string]string:
		return m
	default:
		return nil
	}
}

func withFieldVar(field string, meta map[string]string) map[string]string {
	if field == "" {
		return meta
	}
	out := make(map[string]string, len(meta)+1)
	maps.Copy(out, meta)
	if _, exists := out["field"]; !exists {
		out["field"] = field
	}
	return out
}

type noopTranslator struct{}

func (noopTranslator) Match(string, string) language.Tag { return language.English }
func (noopTranslator) T(language.Tag, string, map[string]string) (string, language.Tag, bool) {
	return "", language.English, false
}
func (noopTranslator) Default() language.Tag { return language.English }
