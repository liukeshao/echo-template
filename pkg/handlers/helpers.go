package handlers

import (
	"encoding/json"
	"io"

	"github.com/labstack/echo/v5"

	"github.com/liukeshao/echo-template/pkg/patch"
)

type validatable interface {
	Validate() error
}

// bindAndValidate 绑定请求参数/主体并执行 Validate()。
func bindAndValidate[T any, PT interface {
	*T
	validatable
}](c *echo.Context) (T, error) {
	var req T
	if err := c.Bind(&req); err != nil {
		return req, err
	}
	if err := PT(&req).Validate(); err != nil {
		return req, err
	}
	return req, nil
}

// decodePatch 将当前资源视图序列化为 JSON，按请求 Content-Type 应用多协议 PATCH，
// 再反序列化为目标替换请求类型 T（只读字段自动丢弃）。
func decodePatch[T any](c *echo.Context, current any) (T, error) {
	var req T
	currentJSON, err := json.Marshal(current)
	if err != nil {
		return req, err
	}
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return req, err
	}
	if err := patch.ApplyInto(currentJSON, body, c.Request().Header.Get(echo.HeaderContentType), &req); err != nil {
		return req, err
	}
	return req, nil
}

// mapSlice 将源切片逐元素映射为目标切片，保证返回值永不为 nil（空切片 JSON 序列化为 []）。
func mapSlice[In, Out any](in []In, fn func(In) Out) []Out {
	out := make([]Out, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}
