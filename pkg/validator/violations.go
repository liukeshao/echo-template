// Package validator 提供请求参数的代码级校验能力。
//
// 与 i18n 的对接：
//   - FieldViolations 以 (field, key, vars) 的抽象形式收集错误；
//     key 为稳定的翻译键（如 "validation.required"、"validation.max_length"）。
//   - vars 用于模板占位符（{field}、{max} 等）。
//   - 渲染与本地化由 handler 层的 error_handler 通过 i18n.Translator 完成，
//     validator 包不直接产生面向用户的字符串，从而获得完整的 i18n 能力。
package validator

import (
	"fmt"
	"maps"
	"strings"
)

// Violation 是 FieldViolations 的内部表示，对应一条字段级校验错误。
//
// 面向 wire 的形式是 errdetails.FieldViolation，其 Description 由
// error_handler 在渲染阶段基于 Translator 填充。
type Violation struct {
	// Field 为 JSON 字段名；空字符串表示跨字段错误（如 "at least one field must be provided"）。
	Field string
	// Key 为 i18n 翻译键，例如 "validation.required"。
	Key string
	// Vars 是 {name} 占位符到具体值的映射。可为 nil。
	Vars map[string]string
}

// FieldViolations 收集字段级校验错误列表，实现 error 接口。
// 在各 ValidateXxx 函数中声明为零值，逐条 Add，最终 Err() 返回。
type FieldViolations struct {
	items []Violation
}

// Add 追加一个字段校验错误，返回自身以支持链式调用。
//
// field 为 JSON 字段名；key 为翻译键；vars 为模板占位符映射（可为 nil）。
// vars 会被浅拷贝一次以避免调用方后续改动意外影响错误内容。
func (v *FieldViolations) Add(field, key string, vars map[string]string) *FieldViolations {
	var copied map[string]string
	if len(vars) > 0 {
		copied = make(map[string]string, len(vars))
		maps.Copy(copied, vars)
	}
	v.items = append(v.items, Violation{
		Field: field,
		Key:   key,
		Vars:  copied,
	})
	return v
}

// HasErrors 报告是否存在至少一个校验错误。
func (v *FieldViolations) HasErrors() bool {
	return len(v.items) > 0
}

// Items 返回所有字段校验错误的只读副本。
// 调用方应视为只读；不应修改返回切片。
func (v *FieldViolations) Items() []Violation {
	if len(v.items) == 0 {
		return nil
	}
	out := make([]Violation, len(v.items))
	copy(out, v.items)
	return out
}

// Error 实现 error 接口，返回调试用的非本地化拼接字符串。
// 输出格式为 "field: key{k=v,...}; ..."，**不应作为面向用户的消息使用**；
// 面向用户的本地化消息由 error_handler 通过 i18n.Translator 生成。
func (v *FieldViolations) Error() string {
	parts := make([]string, 0, len(v.items))
	for _, it := range v.items {
		var varStr string
		if len(it.Vars) > 0 {
			pieces := make([]string, 0, len(it.Vars))
			for k, vv := range it.Vars {
				pieces = append(pieces, fmt.Sprintf("%s=%s", k, vv))
			}
			varStr = "{" + strings.Join(pieces, ",") + "}"
		}
		if it.Field != "" {
			parts = append(parts, fmt.Sprintf("%s: %s%s", it.Field, it.Key, varStr))
		} else {
			parts = append(parts, fmt.Sprintf("%s%s", it.Key, varStr))
		}
	}
	return strings.Join(parts, "; ")
}

// Err 当存在错误时返回 *FieldViolations（作为 error），否则返回 nil。
// 通常在 ValidateXxx 末尾调用：return v.Err()
func (v *FieldViolations) Err() error {
	if v.HasErrors() {
		return v
	}
	return nil
}
