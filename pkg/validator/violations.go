// Package validator 提供零依赖、零反射的结构化字段校验原语。
//
// 与 i18n 的对接：
//   - FieldViolations 以 (field, Rule, metadata) 的纯结构化形式收集错误；
//     Rule 为强类型校验规则码（如 RuleRequired、RuleMaxLength）。
//   - metadata 保存规则阈值等机器可读元数据（如 {"max": "200"}），同时供 HTTP 响应透出与 i18n 模板插值。
//   - validator 包绝不产生面向用户的自然语言字符串，统一由出口 ErrorEncoder 本地化渲染。
package validator

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Rule 是强类型校验规则码，既作为 httperr.FieldError.Code 对外暴露，
// 又与 internal/i18n/messages/*.toml 中 [validation] 节的键名 1:1 对应。
// 业务层可按需定义自己的 Rule 常量并配合 FieldViolations.Add 使用。
type Rule string

// Key 返回 i18n 查表键名（即规则码本身）。
func (r Rule) Key() string { return string(r) }

// String 实现 fmt.Stringer。
func (r Rule) String() string { return string(r) }

// Param 表示一条结构化校验约束参数（如 P("max", "200")），用于替代裸 map[string]string。
type Param struct {
	Key string
	Val string
}

// P 构造一条校验约束参数。
func P(key, val string) Param {
	return Param{Key: key, Val: val}
}

// Violation 是 FieldViolations 的内部表示，对应一条字段级校验错误。
//
// 面向 wire 的形式是 httperr.FieldError，其 Message 在 HTTP 出口基于 Translator 统一填充。
type Violation struct {
	// Field 为 JSON/Query/Path 字段名；空字符串表示跨字段/请求级校验错误。
	Field string
	// Code 为强类型机器可读规则码，与 internal/i18n/messages/*.toml 中 [validation] 键一一对应。
	Code Rule
	// Metadata 是结构化约束元数据（不含冗余 "field" 键），同时返回给客户端并用于模板插值。可为 nil。
	Metadata map[string]string
}

// FieldViolations 收集字段级校验错误列表，实现 error 接口。
// 在各 Validate() 方法中声明为零值，逐条校验后最终通过 Err() 返回。
type FieldViolations struct {
	items []Violation
}

// Add 追加一条字段校验错误。无约束参数时直接省略 params，无需传 nil。
func (v *FieldViolations) Add(field string, rule Rule, params ...Param) {
	var meta map[string]string
	if len(params) > 0 {
		meta = make(map[string]string, len(params))
		for _, p := range params {
			if p.Key != "" {
				meta[p.Key] = p.Val
			}
		}
		if len(meta) == 0 {
			meta = nil
		}
	}
	v.items = append(v.items, Violation{
		Field:    field,
		Code:     rule,
		Metadata: meta,
	})
}

// HasErrors 报告是否存在至少一个校验错误。
func (v *FieldViolations) HasErrors() bool {
	return len(v.items) > 0
}

// Items 返回所有字段校验错误的只读深拷贝，避免调用方修改 Metadata 影响内部状态。
func (v *FieldViolations) Items() []Violation {
	if len(v.items) == 0 {
		return nil
	}
	out := make([]Violation, len(v.items))
	for i, it := range v.items {
		if len(it.Metadata) > 0 {
			it.Metadata = maps.Clone(it.Metadata)
		}
		out[i] = it
	}
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
