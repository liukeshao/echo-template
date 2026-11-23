// Package httpruntime 提供基于 Go 1.22+ 标准库 net/http 的极薄 REST 运行时，
// 配合 TypeSpec Emitter 生成的 MountXxxHandler 实现 Connect-Go v2 风格的零反射协议层。
package httpruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/i18n"
	"github.com/liukeshao/echo-template/pkg/patch"
	"github.com/liukeshao/echo-template/pkg/validator"
)

// Runtime 封装 HTTP 编解码、多协议 PATCH 与 RFC 9457 错误渲染。
type Runtime struct {
	translator i18n.Translator
}

// New 使用给定的 i18n.Translator 创建 Runtime 实例。
func New(t i18n.Translator) *Runtime {
	return &Runtime{translator: t}
}

// NewFromInjector 供 DI 容器构造 *Runtime。
func NewFromInjector(i do.Injector) (*Runtime, error) {
	return New(do.MustInvoke[i18n.Translator](i)), nil
}

// DecodeJSON 从请求体解码 JSON；请求体超限时返回 413，语法/类型错误返回 400 Problem。
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
		return Problem{
			Status: http.StatusBadRequest,
			Code:   "BAD_REQUEST",
			Detail: fmt.Sprintf("Invalid JSON request body: %v", err),
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
		v.Add(name, validator.KeyNonNegative, map[string]string{"field": name})
		return 0
	}
	return int32(n)
}

// ApplyPatch 将 current 序列化后按 Content-Type 应用多协议 PATCH，并反序列化到 dst。
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
	return patch.ApplyInto(currentJSON, body, r.Header.Get("Content-Type"), dst)
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
