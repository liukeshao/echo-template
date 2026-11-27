package httperr

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"strconv"
)

// RequestIDFunc 从请求上下文提取追踪 ID，用于填充 Problem.RequestID。
type RequestIDFunc func(ctx context.Context) string

// MetadataExtractor 从自定义错误链（例如 samber/oops）中提取对外公开的强类型 Metadata。
// 仅在 4xx 响应且 Problem.Metadata 为空时调用。
type MetadataExtractor func(err error) (Metadata, bool)

// TranslateFunc 按请求上下文翻译指定 key 与占位符变量：
//   - text: 渲染后的文案；
//   - lang: 实际命中的 BCP-47 语言标签（如 "en"、"zh-CN"），用于写入 Content-Language 响应头；
//   - ok:   是否命中翻译。
type TranslateFunc func(ctx context.Context, key string, vars map[string]string) (text, lang string, ok bool)

// EncoderOption 配置由 NewEncoder 构造的 RFC 9457 错误编码器。
type EncoderOption func(*encoder)

// WithRequestID 注入请求追踪 ID 提取器（例如 pkg/logging.RequestID）。
func WithRequestID(fn RequestIDFunc) EncoderOption {
	return func(e *encoder) {
		e.requestID = fn
	}
}

// WithMetadataExtractor 注入自定义错误链元数据提取器（例如从 oops.Context()[httperr.MetaKey] 提取）。
func WithMetadataExtractor(fn MetadataExtractor) EncoderOption {
	return func(e *encoder) {
		e.extractMetadata = fn
	}
}

// WithTranslator 注入终端分层国际化翻译回调：
// 按 details.<STATUS>.<subkey> → errors.<STATUS>（5xx 兜底 errors.500）与 validation.<code> 查表，
// 并自动写入 Content-Language 响应头。
func WithTranslator(fn TranslateFunc) EncoderOption {
	return func(e *encoder) {
		e.translate = fn
	}
}

// NewEncoder 构造遵循标准三段式流水线的 RFC 9457 Encoder：
//  1. Classify：优先识别请求自身 ctx 取消/超时（499/504），再经 Classify 归类，4xx 时按需提取 Metadata；
//  2. Sanitize & Log：注入 RequestID；5xx 清空 Detail/Metadata/Errors 并记 Error 日志，4xx 记 Debug 日志；
//  3. Localize & Write：执行分层本地化并同步 Content-Language，最后经 Write 输出响应（含 Retry-After）。
func NewEncoder(opts ...EncoderOption) Encoder {
	enc := &encoder{}
	for _, opt := range opts {
		if opt != nil {
			opt(enc)
		}
	}
	return enc.encode
}

var defaultEncoder = &encoder{}

// DefaultEncoder 是零配置的默认 RFC 9457 编码器（等价于 NewEncoder()），
// 适合独立单测或轻量服务开箱即用。
func DefaultEncoder(w http.ResponseWriter, r *http.Request, err error) {
	defaultEncoder.encode(w, r, err)
}

type encoder struct {
	requestID       RequestIDFunc
	extractMetadata MetadataExtractor
	translate       TranslateFunc
}

func (e *encoder) encode(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
	}

	// Stage 1: 单一路径分类与强类型 Metadata 提取
	p := e.classify(ctx, err)

	// Stage 2: 安全隔离与结构化日志
	if p.RequestID == "" && e.requestID != nil {
		p.RequestID = e.requestID(ctx)
	}
	if p.Status >= http.StatusInternalServerError {
		p.Detail = ""
		p.Metadata = nil
		p.Errors = nil
		slog.ErrorContext(ctx, "unhandled error", "status", p.Status, "error", err)
	} else {
		slog.DebugContext(ctx, "client error", "status", p.Status, "error", err)
	}

	// Stage 3: 终端统一分层国际化，并同步 Content-Language 响应头
	if lang := e.localize(ctx, &p); lang != "" {
		w.Header().Set("Content-Language", lang)
	}

	Write(w, r, p)
}

func (e *encoder) classify(ctx context.Context, err error) Problem {
	// 请求自身的取消/超时优先归类为 499/504，防止 DB/下游操作中途因客户端断开被误报 500 告警。
	if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
		return Classify(ctxErr)
	}
	p := Classify(err)
	if p.Metadata == nil && p.Status < http.StatusInternalServerError && e.extractMetadata != nil {
		if m, ok := e.extractMetadata(err); ok {
			p.Metadata = m
		}
	}
	return p
}

func (e *encoder) localize(ctx context.Context, p *Problem) string {
	var usedLang string
	var detail string

	if e.translate != nil && p.Status > 0 {
		statusKey := strconv.Itoa(p.Status)
		var (
			vars    map[string]string
			subkeys []string
		)
		if m, ok := p.Metadata.(Metadata); ok {
			vars = m.I18nVars()
			subkeys = m.I18nSubkeys()
		}
		for _, sub := range subkeys {
			if msg, lang, ok := e.translate(ctx, "details."+statusKey+"."+sub, vars); ok {
				detail = msg
				usedLang = lang
				break
			}
		}
		if detail == "" {
			if msg, lang, ok := e.translate(ctx, "errors."+statusKey, vars); ok {
				detail = msg
				usedLang = lang
			}
		}
		if detail == "" && p.Status >= http.StatusInternalServerError {
			if msg, lang, ok := e.translate(ctx, "errors."+Internal.Key(), nil); ok {
				detail = msg
				usedLang = lang
			}
		}
	}
	if detail != "" {
		p.Detail = detail
	} else if p.Detail == "" {
		p.Detail = StatusTitle(p.Status)
	}

	for i := range p.Errors {
		fe := &p.Errors[i]
		if fe.Code == "" {
			continue
		}
		if e.translate != nil {
			vars := withFieldVar(fe.Field, fe.Metadata)
			if msg, lang, ok := e.translate(ctx, "validation."+fe.Code, vars); ok {
				fe.Message = msg
				if usedLang == "" {
					usedLang = lang
				}
				continue
			}
		}
		if fe.Message == "" {
			fe.Message = fe.Code
		}
	}
	return usedLang
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
