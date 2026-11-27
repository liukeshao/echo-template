package validator

import (
	"slices"
	"strings"
)

// OrderTerm 表示单条已通过白名单校验的排序项（Field 已规范化为小写）。
type OrderTerm struct {
	Field string
	Desc  bool
}

// ParseOrderBy 解析逗号分隔的排序表达式（如 "price desc, title"），
// 仅接受 allowed 白名单中的字段名（大小写不敏感）。
//
// 行为约定：
//   - raw 为空或仅含空白与逗号时返回 (nil, nil)，由调用方应用默认排序；
//   - 每段仅允许 "<field>"（升序）或 "<field> desc"（降序）；
//   - 存在任意非法片段时收集全部错误并返回 *FieldViolations。
func ParseOrderBy(field, raw string, allowed ...string) ([]OrderTerm, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	allowedLower := make([]string, len(allowed))
	for i, a := range allowed {
		allowedLower[i] = strings.ToLower(strings.TrimSpace(a))
	}
	allowedDisplay := strings.Join(allowedLower, ", ")

	var (
		terms []OrderTerm
		v     FieldViolations
	)
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		tokens := strings.Fields(part)
		if len(tokens) > 2 {
			v.Add(field, RuleInvalidOrderByClause, P("clause", part))
			continue
		}

		name := strings.ToLower(tokens[0])
		desc := len(tokens) == 2 && strings.ToLower(tokens[1]) == "desc"
		if len(tokens) == 2 && !desc {
			v.Add(field, RuleInvalidOrderByDirection,
				P("direction", tokens[1]),
				P("clause", part),
			)
			continue
		}

		if !slices.Contains(allowedLower, name) {
			v.Add(field, RuleUnknownOrderByField,
				P("value", name),
				P("allowed", allowedDisplay),
			)
			continue
		}

		terms = append(terms, OrderTerm{
			Field: name,
			Desc:  desc,
		})
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	return terms, nil
}
