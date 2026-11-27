package httperr

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"strconv"
)

type RequestIDFunc func(ctx context.Context) string

// MetadataExtractor 从自定义错误链中提取公开 Metadata（仅在 4xx 且 Problem.Metadata 为空时调用）。
type MetadataExtractor func(err error) (Metadata, bool)

// TranslateFunc 翻译指定 key，返回渲染文本、实际命中的 BCP-47 语言标签及是否命中。
type TranslateFunc func(ctx context.Context, key string, vars map[string]string) (text, lang string, ok bool)

type EncoderOption func(*encoder)

func WithRequestID(fn RequestIDFunc) EncoderOption {
	return func(e *encoder) {
		e.requestID = fn
	}
}

func WithMetadataExtractor(fn MetadataExtractor) EncoderOption {
	return func(e *encoder) {
		e.extractMetadata = fn
	}
}

func WithTranslator(fn TranslateFunc) EncoderOption {
	return func(e *encoder) {
		e.translate = fn
	}
}

// NewEncoder 构造 RFC 9457 错误编码器（归类 → 脱敏与日志 → 本地化写出）。
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

	p := e.classify(ctx, err)

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
