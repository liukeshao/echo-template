package validator

import (
	"slices"
	"strings"
)

type OrderTerm struct {
	Field string
	Desc  bool
}

// ParseOrderBy 解析逗号分隔的排序表达式（"<field>"、"<field> asc" 或 "<field> desc"），并按 allowed 白名单校验。
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
		var desc bool
		if len(tokens) == 2 {
			switch strings.ToLower(tokens[1]) {
			case "asc":
				desc = false
			case "desc":
				desc = true
			default:
				v.Add(field, RuleInvalidOrderByDirection,
					P("direction", tokens[1]),
					P("clause", part),
				)
				continue
			}
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
