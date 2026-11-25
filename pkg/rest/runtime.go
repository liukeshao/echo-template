// Package rest 提供基于 Go 1.22+ 标准库 net/http 的轻量 REST 运行时，
// 配合 TypeSpec Emitter 生成的 api.NewXxxHandler 实现 Connect-Go v2 风格的零反射协议层。
package rest

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/samber/do/v2"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/pkg/i18n"
	"github.com/liukeshao/echo-template/pkg/validator"
)

// Runtime 封装 HTTP 编解码、RFC 7386 JSON Merge Patch 与 RFC 9457 错误渲染。
type Runtime struct {
	translator i18n.Translator
}

// New 使用给定的 i18n.Translator 创建 Runtime 实例；若为 nil 则使用英文兜底实现。
func New(t i18n.Translator) *Runtime {
	if t == nil {
		t = noopTranslator{}
	}
	return &Runtime{translator: t}
}

// DefaultRuntime 返回不依赖外部 i18n 配置的默认 Runtime（适合独立单测）。
func DefaultRuntime() *Runtime {
	return New(nil)
}

// NewFromInjector 供 DI 容器构造 *Runtime。
func NewFromInjector(i do.Injector) (*Runtime, error) {
	return New(do.MustInvoke[i18n.Translator](i)), nil
}

// DecodeJSON 从请求体解码 JSON；请求体超限时返回 *http.MaxBytesError（→ 413），
// 字段类型不匹配返回 *validator.FieldViolations（不泄漏内部 Go 类型名），语法错误返回本地化 400 BAD_REQUEST。
func (rt *Runtime) DecodeJSON(r *http.Request, dst any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return maxErr
		}
		return err
	}
	if err := json.Unmarshal(body, dst); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			var v validator.FieldViolations
			v.Add(typeErr.Field, validator.KeyInvalidType, map[string]string{"field": typeErr.Field})
			return v.Err()
		}
		return Problem{
			Status: http.StatusBadRequest,
			Code:   "BAD_REQUEST",
		}
	}
	return nil
}

// ParseInt32Query 零反射解析 int32 query 参数；非法数字直接记录为字段校验错误。
func (rt *Runtime) ParseInt32Query(q url.Values, name string, v *validator.FieldViolations) int32 {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		v.Add(name, validator.KeyInteger, map[string]string{"field": name})
		return 0
	}
	return int32(n)
}

// ParseInt64Query 零反射解析 int64 query 参数；非法数字直接记录为字段校验错误。
func (rt *Runtime) ParseInt64Query(q url.Values, name string, v *validator.FieldViolations) int64 {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		v.Add(name, validator.KeyInteger, map[string]string{"field": name})
		return 0
	}
	return n
}

// ParseFloat64Query 零反射解析 float64 query 参数；非法数字（含 NaN/Inf）直接记录为字段校验错误。
func (rt *Runtime) ParseFloat64Query(q url.Values, name string, v *validator.FieldViolations) float64 {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		v.Add(name, validator.KeyNumber, map[string]string{"field": name})
		return 0
	}
	return n
}

// ParseOptionalFloat64Query 零反射解析可选 *float64 query 参数；未传返回 nil，以区分 0 与未提供。
func (rt *Runtime) ParseOptionalFloat64Query(q url.Values, name string, v *validator.FieldViolations) *float64 {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return nil
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		v.Add(name, validator.KeyNumber, map[string]string{"field": name})
		return nil
	}
	return &n
}

// ParseBoolQuery 零反射解析 bool query 参数；非法布尔值直接记录为字段校验错误。
func (rt *Runtime) ParseBoolQuery(q url.Values, name string, v *validator.FieldViolations) bool {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return false
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		v.Add(name, validator.KeyBoolean, map[string]string{"field": name})
		return false
	}
	return b
}

// ParseOptionalBoolQuery 零反射解析可选 *bool query 参数；未传返回 nil，以区分 false 与未提供。
func (rt *Runtime) ParseOptionalBoolQuery(q url.Values, name string, v *validator.FieldViolations) *bool {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		v.Add(name, validator.KeyBoolean, map[string]string{"field": name})
		return nil
	}
	return &b
}

// ApplyPatch 将 current 序列化后按 RFC 7386 JSON Merge Patch 合并请求体，并反序列化到 dst。
func (rt *Runtime) ApplyPatch(r *http.Request, current any, dst any) error {
	currentJSON, err := json.Marshal(current)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return maxErr
		}
		return err
	}
	return ApplyMergePatchInto(currentJSON, body, r.Header.Get("Content-Type"), dst)
}

// WriteJSON 写出 JSON 成功响应。
func (rt *Runtime) WriteJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.Error("failed to marshal JSON response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// WriteNoContent 写出 204 No Content。
func (rt *Runtime) WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

type noopTranslator struct{}

func (noopTranslator) Match(string, string) language.Tag { return language.English }
func (noopTranslator) T(language.Tag, string, map[string]string) (string, language.Tag, bool) {
	return "", language.English, false
}
func (noopTranslator) Default() language.Tag { return language.English }
