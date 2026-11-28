// Package validator 提供零依赖、零反射的结构化字段校验原语，不生成面向用户的自然语言文案。
package validator

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Rule 为强类型校验规则码，与 i18n [validation] 键一一对应。
type Rule string

func (r Rule) Key() string    { return string(r) }
func (r Rule) String() string { return string(r) }

type Param struct {
	Key string
	Val string
}

func P(key, val string) Param {
	return Param{Key: key, Val: val}
}

type Violation struct {
	Field    string
	Code     Rule
	Metadata map[string]string
}

type FieldViolations struct {
	items []Violation
}

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

// Merge 将嵌套结构体或切片元素的 *FieldViolations 按 prefix 路径前缀合并到当前集合
// （如 prefix="address" → "address.city"，prefix="items[0]" → "items[0].sku"）。
func (v *FieldViolations) Merge(prefix string, err error) {
	if err == nil {
		return
	}
	fv, ok := errors.AsType[*FieldViolations](err)
	if !ok || fv == nil {
		return
	}
	for _, it := range fv.items {
		field := it.Field
		switch {
		case prefix == "":
		case field == "":
			field = prefix
		case strings.HasPrefix(field, "["):
			field = prefix + field
		default:
			field = prefix + "." + field
		}
		var meta map[string]string
		if len(it.Metadata) > 0 {
			meta = maps.Clone(it.Metadata)
		}
		v.items = append(v.items, Violation{
			Field:    field,
			Code:     it.Code,
			Metadata: meta,
		})
	}
}

func (v *FieldViolations) HasErrors() bool {
	return len(v.items) > 0
}

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

func (v *FieldViolations) Err() error {
	if v.HasErrors() {
		return v
	}
	return nil
}
