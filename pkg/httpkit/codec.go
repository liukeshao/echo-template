package httpkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/liukeshao/echo-template/pkg/httperr"
	"github.com/liukeshao/echo-template/pkg/validator"
)

// DecodeJSON 从请求体解码单个 JSON 值；
//   - 请求体超限时原样返回 *http.MaxBytesError（→ 413 + RequestLimitInfo）
//   - 字段类型不匹配时返回 *validator.FieldViolations（附带 expected/actual JSON 类型，不泄漏 Go 类型名）
//   - 空主体、语法错误或尾随多余 JSON 时返回 400 Problem（附带 DecodeInfo）
func DecodeJSON(r *http.Request, dst any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if maxErr, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return maxErr
		}
		return err
	}

	if len(bytes.TrimSpace(body)) == 0 {
		return httperr.Problem{
			Status:   http.StatusBadRequest,
			Metadata: httperr.DecodeInfo{Reason: httperr.DecodeEmptyBody},
		}
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(dst); err != nil {
		if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok && typeErr.Field != "" {
			var v validator.FieldViolations
			params := []validator.Param{validator.P("expected", jsonValueKind(typeErr.Type))}
			if typeErr.Value != "" {
				params = append(params, validator.P("actual", typeErr.Value))
			}
			v.Add(typeErr.Field, validator.RuleInvalidType, params...)
			return v.Err()
		}
		if synErr, ok := errors.AsType[*json.SyntaxError](err); ok {
			return httperr.Problem{
				Status: http.StatusBadRequest,
				Metadata: httperr.DecodeInfo{
					Reason: httperr.DecodeInvalidJSON,
					Offset: synErr.Offset,
				},
			}
		}
		return httperr.Problem{
			Status:   http.StatusBadRequest,
			Metadata: httperr.DecodeInfo{Reason: httperr.DecodeInvalidJSON},
		}
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return httperr.Problem{
			Status:   http.StatusBadRequest,
			Metadata: httperr.DecodeInfo{Reason: httperr.DecodeTrailingData},
		}
	}
	return nil
}

// jsonValueKind 将 encoding/json 已填充的字段目标类型映射为安全的通用 JSON 类型名，绝不暴露 Go 包名或结构体名。
func jsonValueKind(t reflect.Type) string {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil {
		return "value"
	}
	if t == reflect.TypeFor[time.Time]() {
		return "string"
	}
	switch t.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.String:
		return "string"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Struct, reflect.Map:
		return "object"
	default:
		return "value"
	}
}

// ParseInt32Query 零反射解析 int32 query 参数；非法数字直接记录为字段校验错误。
func ParseInt32Query(q url.Values, name string, v *validator.FieldViolations) int32 {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		v.Add(name, validator.RuleInteger, validator.P("expected", "int32"))
		return 0
	}
	return int32(n)
}

// ParseInt64Query 零反射解析 int64 query 参数；非法数字直接记录为字段校验错误。
func ParseInt64Query(q url.Values, name string, v *validator.FieldViolations) int64 {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		v.Add(name, validator.RuleInteger, validator.P("expected", "int64"))
		return 0
	}
	return n
}

// ParseFloat64Query 零反射解析 float64 query 参数；非法数字（含 NaN/Inf）直接记录为字段校验错误。
func ParseFloat64Query(q url.Values, name string, v *validator.FieldViolations) float64 {
	if n := ParseOptionalFloat64Query(q, name, v); n != nil {
		return *n
	}
	return 0
}

// ParseOptionalFloat64Query 零反射解析可选 *float64 query 参数；未传返回 nil，以区分 0 与未提供。
func ParseOptionalFloat64Query(q url.Values, name string, v *validator.FieldViolations) *float64 {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return nil
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		v.Add(name, validator.RuleNumber, validator.P("expected", "float64"))
		return nil
	}
	return &n
}

// ParseBoolQuery 零反射解析 bool query 参数；非法布尔值直接记录为字段校验错误。
func ParseBoolQuery(q url.Values, name string, v *validator.FieldViolations) bool {
	if b := ParseOptionalBoolQuery(q, name, v); b != nil {
		return *b
	}
	return false
}

// ParseOptionalBoolQuery 零反射解析可选 *bool query 参数；未传返回 nil，以区分 false 与未提供。
func ParseOptionalBoolQuery(q url.Values, name string, v *validator.FieldViolations) *bool {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		v.Add(name, validator.RuleBoolean, validator.P("expected", "boolean"))
		return nil
	}
	return &b
}

// ApplyPatch 将 current 序列化后按 RFC 7386 JSON Merge Patch 合并请求体，并反序列化到 dst。
func ApplyPatch(r *http.Request, current any, dst any) error {
	currentJSON, err := json.Marshal(current)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if maxErr, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return maxErr
		}
		return err
	}
	return ApplyMergePatchInto(currentJSON, body, r.Header.Get("Content-Type"), dst)
}

// WriteJSON 写出 JSON 成功响应；序列化失败时记录带上下文的错误日志并返回 500。
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to marshal JSON response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// WriteNoContent 写出 204 No Content。
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
