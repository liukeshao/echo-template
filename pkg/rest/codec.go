package rest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/liukeshao/echo-template/pkg/validator"
)

// DecodeJSON 从请求体解码单个 JSON 值；
//   - 请求体超限时原样返回 *http.MaxBytesError（→ 413）
//   - 字段类型不匹配时返回 *validator.FieldViolations（不泄漏内部 Go 类型名）
//   - 空主体、语法错误或尾随多余 JSON 时返回 400 BAD_REQUEST Problem
func DecodeJSON(r *http.Request, dst any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if maxErr, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return maxErr
		}
		return err
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(dst); err != nil {
		if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok && typeErr.Field != "" {
			var v validator.FieldViolations
			v.Add(typeErr.Field, validator.KeyInvalidType, map[string]string{"field": typeErr.Field})
			return v.Err()
		}
		return Problem{
			Status: http.StatusBadRequest,
			Code:   CodeBadRequest,
		}
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return Problem{
			Status: http.StatusBadRequest,
			Code:   CodeBadRequest,
		}
	}
	return nil
}

// ParseInt32Query 零反射解析 int32 query 参数；非法数字直接记录为字段校验错误。
func ParseInt32Query(q url.Values, name string, v *validator.FieldViolations) int32 {
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
func ParseInt64Query(q url.Values, name string, v *validator.FieldViolations) int64 {
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
func ParseFloat64Query(q url.Values, name string, v *validator.FieldViolations) float64 {
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
func ParseOptionalFloat64Query(q url.Values, name string, v *validator.FieldViolations) *float64 {
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
func ParseBoolQuery(q url.Values, name string, v *validator.FieldViolations) bool {
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
func ParseOptionalBoolQuery(q url.Values, name string, v *validator.FieldViolations) *bool {
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
