package server

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"net/http"

	"github.com/samber/oops"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/internal/i18n"
	"github.com/liukeshao/echo-template/pkg/logging"
	"github.com/liukeshao/echo-template/pkg/rest"
)

// NewProblemEncoder 返回应用级 RFC 9457 ErrorEncoder，按三段式流水线处理所有错误：
//  1. Classify：优先识别请求上下文取消/超时，再提取 oops 的有限 rest.Code + 强类型 Metadata，其余委托 rest.Classify
//  2. Sanitize & Log：注入 RequestID；5xx 记录 Error 日志并清空对外 Metadata/Errors，4xx 记录 Debug 日志
//  3. Unified Localize：唯一国际化出口，按 details.<CODE>.<subkey> → errors.<CODE> 分层回落渲染并写入 Content-Language
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

	// Stage 1: 纯分类与强类型 Metadata 提取（零自然语言文案，不读取 oops.Public）
	p := classifyError(ctx, err)

	// Stage 2: 安全隔离与结构化日志
	if p.RequestID == "" {
		p.RequestID = logging.RequestID(ctx)
	}
	if p.Status >= http.StatusInternalServerError {
		p.Metadata = nil
		p.Errors = nil
		slog.ErrorContext(ctx, "unhandled error", "status", p.Status, "code", p.Code, "error", err)
	} else {
		slog.DebugContext(ctx, "client error", "status", p.Status, "code", p.Code, "error", err)
	}

	// Stage 3: 终端统一分层国际化，并同步 Content-Language 响应头
	if used := e.localizeProblem(&p, loc); used != language.Und {
		w.Header().Set("Content-Language", used.String())
	}

	rest.WriteProblem(w, r, p)
}

func classifyError(ctx context.Context, err error) rest.Problem {
	// 请求自身的取消/超时优先归类为 499/504，防止 DB/下游操作中途因客户端断开被误报 500 告警。
	if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
		return rest.Classify(ctxErr)
	}
	if oopsErr, ok := oops.AsOops(err); ok {
		if code, ok := rest.CodeFromAny(oopsErr.Code()); ok {
			status := code.HTTPStatus()
			var meta rest.Metadata
			if status < http.StatusInternalServerError {
				if m, ok := oopsErr.Context()[rest.MetaKey].(rest.Metadata); ok {
					meta = m
				}
			}
			return rest.Problem{
				Status:   status,
				Code:     code,
				Metadata: meta,
			}
		}
	}
	return rest.Classify(err)
}

func (e *problemEncoder) localizeProblem(p *rest.Problem, loc language.Tag) language.Tag {
	used := language.Und
	if p.Code != "" {
		vars, subkeys := metadataI18n(p.Metadata)
		for _, sub := range subkeys {
			if msg, lang, ok := e.translator.T(loc, "details."+p.Code.String()+"."+sub, vars); ok {
				p.Detail = msg
				used = lang
				break
			}
		}
		if p.Detail == "" {
			if msg, lang, ok := e.translator.T(loc, "errors."+p.Code.String(), vars); ok {
				p.Detail = msg
				used = lang
			}
		}
	}
	if p.Detail == "" && p.Status >= http.StatusInternalServerError {
		if msg, lang, ok := e.translator.T(loc, "errors."+rest.CodeInternal.String(), nil); ok {
			p.Detail = msg
			used = lang
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
		if msg, lang, ok := e.translator.T(loc, "validation."+fe.Code, vars); ok {
			fe.Message = msg
			if used == language.Und {
				used = lang
			}
		} else if fe.Message == "" {
			fe.Message = fe.Code
		}
	}
	return used
}

func metadataI18n(meta any) (map[string]string, []string) {
	switch m := meta.(type) {
	case rest.Metadata:
		return m.I18nVars(), m.I18nSubkeys()
	case map[string]string:
		return m, nil
	default:
		return nil, nil
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
