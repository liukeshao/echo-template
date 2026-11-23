package codes

import (
	"fmt"
	"net/http"
)

// CtxKeyParams 是 oops context key，用于向 i18n 模板与 RFC 9457 Problem.Params
// 传递动态键值对，值类型为 map[string]string。
const CtxKeyParams = "params"

// Code 是机器可读的业务错误码类型（UPPER_SNAKE_CASE）。
type Code string

// String 实现 fmt.Stringer，供 oops.Code(c.String()) 使用。
func (c Code) String() string { return string(c) }

// registry 存储业务错误码到 HTTP 状态码的映射。
// 各领域在自己文件的 init() 中调用 Register，实现去中心化注册。
var registry = map[Code]int{}

// Register 注册业务错误码对应的 HTTP 状态码（4xx / 5xx）。
// 在各领域文件的 init() 中调用，保证程序启动时完成注册。
// 重复注册同一 Code 或状态码不在 400–599 范围内时 panic，防止配置错误静默生效。
func Register(c Code, httpStatus int) {
	if _, exists := registry[c]; exists {
		panic(fmt.Sprintf("codes: duplicate registration for %q", c))
	}
	if httpStatus < 400 || httpStatus > 599 {
		panic(fmt.Sprintf("codes: invalid HTTP error status %d for %q", httpStatus, c))
	}
	registry[c] = httpStatus
}

// HTTPStatus 返回业务错误码对应的 HTTP 状态码。
// 若未注册则回退到 500 Internal Server Error。
func HTTPStatus(c Code) int {
	if status, ok := registry[c]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// RegisteredCodes 返回所有已通过 Register 注册的业务错误码副本。
// 顺序不稳定；调用方如需稳定顺序请自行排序。
// 主要供 i18n loader 在启动时校验翻译覆盖率使用。
func RegisteredCodes() []Code {
	out := make([]Code, 0, len(registry))
	for c := range registry {
		out = append(out, c)
	}
	return out
}
