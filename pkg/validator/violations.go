// Package validator 提供请求参数的代码级校验能力。
//
// 与 i18n 的对接：
//   - FieldViolations 以 (field, code, metadata) 的纯结构化形式收集错误；
//     code 为稳定的校验规则码（如 "required"、"max_length"）。
//   - metadata 保存规则阈值等机器可读元数据（如 {"max": "200"}），同时供 HTTP 响应透出与 i18n 模板插值。
//   - validator 包绝不产生面向用户的自然语言字符串，统一由顶层 ErrorEncoder 本地化渲染。
package validator

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Violation 是 FieldViolations 的内部表示，对应一条字段级校验错误。
//
// 面向 wire 的形式是 rest.FieldError，其 Message 在 HTTP 出口基于 Translator 统一填充。
type Violation struct {
	// Field 为 JSON/Query/Path 字段名；空字符串表示跨字段/请求级校验错误。
	Field string
	// Code 为机器可读规则码，与 internal/i18n/messages/*.toml 中 [validation] 键一一对应（如 "required"）。
	Code string
	// Metadata 是结构化约束元数据（不含冗余 "field" 键），同时返回给客户端并用于模板插值。可为 nil。
	Metadata map[string]string
}

// FieldViolations 收集字段级校验错误列表，实现 error 接口。
// 在各 Validate() 方法中声明为零值，逐条 Add，最终 Err() 返回。
type FieldViolations struct {
	items []Violation
}

// Add 追加一个字段校验错误，返回自身以支持链式调用。
//
// field 为字段名；code 为规则码；metadata 为结构化约束元数据（可为 nil）。
// metadata 会被浅拷贝一次以避免调用方后续改动意外影响错误内容。
func (v *FieldViolations) Add(field, code string, metadata map[string]string) *FieldViolations {
	var copied map[string]string
	if len(metadata) > 0 {
		copied = make(map[string]string, len(metadata))
		maps.Copy(copied, metadata)
	}
	v.items = append(v.items, Violation{
		Field:    field,
		Code:     code,
		Metadata: copied,
	})
	return v
}

// HasErrors 报告是否存在至少一个校验错误。
func (v *FieldViolations) HasErrors() bool {
	return len(v.items) > 0
}

// Items 返回所有字段校验错误的只读副本。
func (v *FieldViolations) Items() []Violation {
	if len(v.items) == 0 {
		return nil
	}
	out := make([]Violation, len(v.items))
	copy(out, v.items)
	return out
}

// Error 实现 error 接口，返回仅供内部日志与单测调试使用的非本地化摘要。
func (v *FieldViolations) Error() string {
	parts := make([]string, 0, len(v.items))
	for _, it := range v.items {
		var metaStr string
		if len(it.Metadata) > 0 {
			keys := slices.Sorted(maps.Keys(it.Metadata))
			pieces := make([]string, 0, len(keys))
			for _, k := range keys {
				pieces = append(pieces, fmt.Sprintf("%s=%s", k, it.Metadata[k]))
			}
			metaStr = "{" + strings.Join(pieces, ",") + "}"
		}
		if it.Field != "" {
			parts = append(parts, fmt.Sprintf("%s: %s%s", it.Field, it.Code, metaStr))
		} else {
			parts = append(parts, fmt.Sprintf("%s%s", it.Code, metaStr))
		}
	}
	return strings.Join(parts, "; ")
}

// Err 当存在错误时返回 *FieldViolations（作为 error），否则返回 nil。
func (v *FieldViolations) Err() error {
	if v.HasErrors() {
		return v
	}
	return nil
}
