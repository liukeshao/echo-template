package filter

import (
	"fmt"
	"sort"
	"strings"
)

// FilterTranslator translates a filter.Expr AST into domain predicates of type P.
//
// P is typically an ent predicate type (e.g. predicate.Book).
// Construct it once at package level with NewTranslator, then register fields
// via the String / Float64 builder methods. The resulting value is immutable
// and safe for concurrent use.
//
// Example:
//
//	var bookFilter = filter.NewTranslator(book.And, book.Or, book.Not).
//	    String("title",
//	        book.TitleEQ, book.TitleNEQ,
//	        book.TitleLT, book.TitleLTE, book.TitleGT, book.TitleGTE,
//	        book.TitleContainsFold, book.TitleHasPrefix, book.TitleHasSuffix,
//	    ).
//	    Float64("price",
//	        book.PriceEQ, book.PriceNEQ,
//	        book.PriceLT, book.PriceLTE, book.PriceGT, book.PriceGTE,
//	    )
type FilterTranslator[P any] struct {
	and    func(...P) P
	or     func(...P) P
	not    func(P) P
	fields map[string]fieldDef[P]
}

// NewTranslator creates a FilterTranslator with the given AND / OR / NOT combinators.
// The combinators correspond directly to the ent-generated package-level functions,
// e.g. book.And, book.Or, book.Not.
func NewTranslator[P any](
	and func(...P) P,
	or func(...P) P,
	not func(P) P,
) *FilterTranslator[P] {
	return &FilterTranslator[P]{
		and:    and,
		or:     or,
		not:    not,
		fields: make(map[string]fieldDef[P]),
	}
}

// String registers a filterable string field.
// Functions are provided in the order:
//
//	eq, neq  — exact equality / inequality
//	lt, lte, gt, gte — lexicographic comparison
//	contains, hasPrefix, hasSuffix — substring / prefix / suffix match (case-insensitive where available)
//
// The "=" operator supports "*" wildcards in the value:
//
//	"Go*"  → hasPrefix("Go")
//	"*Go"  → hasSuffix("Go")
//	"*Go*" → contains("Go")
func (t *FilterTranslator[P]) String(
	name string,
	eq, neq func(string) P,
	lt, lte, gt, gte func(string) P,
	contains, hasPrefix, hasSuffix func(string) P,
) *FilterTranslator[P] {
	t.fields[name] = &stringField[P]{
		eq: eq, neq: neq,
		lt: lt, lte: lte, gt: gt, gte: gte,
		contains: contains, hasPrefix: hasPrefix, hasSuffix: hasSuffix,
	}
	return t
}

// Float64 registers a filterable float64 field.
// Supported operators: = != < <= > >=. The ":" (Has) operator is not supported.
func (t *FilterTranslator[P]) Float64(
	name string,
	eq, neq func(float64) P,
	lt, lte, gt, gte func(float64) P,
) *FilterTranslator[P] {
	t.fields[name] = &float64Field[P]{
		eq: eq, neq: neq,
		lt: lt, lte: lte, gt: gt, gte: gte,
	}
	return t
}

// Translate converts a filter.Expr into a domain predicate of type P.
// Returns (zero, nil) when expr is nil (empty filter).
// Returns a ValidationInvalid-friendly error for unknown fields or type mismatches.
func (t *FilterTranslator[P]) Translate(expr Expr) (P, error) {
	var zero P
	if expr == nil {
		return zero, nil
	}
	switch e := expr.(type) {
	case *AndExpr:
		l, err := t.Translate(e.Left)
		if err != nil {
			return zero, err
		}
		r, err := t.Translate(e.Right)
		if err != nil {
			return zero, err
		}
		return t.and(l, r), nil

	case *OrExpr:
		l, err := t.Translate(e.Left)
		if err != nil {
			return zero, err
		}
		r, err := t.Translate(e.Right)
		if err != nil {
			return zero, err
		}
		return t.or(l, r), nil

	case *NotExpr:
		p, err := t.Translate(e.Operand)
		if err != nil {
			return zero, err
		}
		return t.not(p), nil

	case *RestrictionExpr:
		fd, ok := t.fields[e.Field]
		if !ok {
			return zero, fmt.Errorf(
				"unknown filter field %q; supported: %s",
				e.Field, t.supportedFields(),
			)
		}
		return fd.translate(e.Op, e.Value)

	default:
		return zero, fmt.Errorf("unsupported expression type %T", expr)
	}
}

// supportedFields returns a sorted, comma-joined list of registered field names.
func (t *FilterTranslator[P]) supportedFields() string {
	names := make([]string, 0, len(t.fields))
	for k := range t.fields {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// ---- field definitions -------------------------------------------------------

// fieldDef is the internal interface implemented by stringField and float64Field.
type fieldDef[P any] interface {
	translate(op Op, val Value) (P, error)
}

// stringField handles string-typed filterable fields.
type stringField[P any] struct {
	eq, neq                        func(string) P
	lt, lte, gt, gte               func(string) P
	contains, hasPrefix, hasSuffix func(string) P
}

func (f *stringField[P]) translate(op Op, val Value) (P, error) {
	var zero P
	sv, ok := extractString(val)
	if !ok {
		return zero, fmt.Errorf("string field requires a string value, got %T", val)
	}
	switch op {
	case OpEQ:
		return f.applyWildcard(sv), nil
	case OpNEQ:
		return f.neq(sv), nil
	case OpLT:
		return f.lt(sv), nil
	case OpLTE:
		return f.lte(sv), nil
	case OpGT:
		return f.gt(sv), nil
	case OpGTE:
		return f.gte(sv), nil
	case OpHas:
		return f.contains(sv), nil
	default:
		return zero, fmt.Errorf("operator %s not supported for string fields", op)
	}
}

// applyWildcard maps "*" wildcards in v to the appropriate predicate:
//
//	"foo*"  → hasPrefix("foo")
//	"*foo"  → hasSuffix("foo")
//	"*foo*" → contains("foo")
//	"fo*o"  → contains("fo")  (middle wildcard falls back to contains)
//	"foo"   → eq("foo")
func (f *stringField[P]) applyWildcard(v string) P {
	if !strings.Contains(v, "*") {
		return f.eq(v)
	}
	leadingStar := strings.HasPrefix(v, "*")
	trailingStar := strings.HasSuffix(v, "*")
	core := strings.ReplaceAll(v, "*", "")
	switch {
	case leadingStar && trailingStar:
		return f.contains(core)
	case trailingStar:
		return f.hasPrefix(core)
	case leadingStar:
		return f.hasSuffix(core)
	default:
		return f.contains(core)
	}
}

// float64Field handles float64-typed filterable fields.
type float64Field[P any] struct {
	eq, neq          func(float64) P
	lt, lte, gt, gte func(float64) P
}

func (f *float64Field[P]) translate(op Op, val Value) (P, error) {
	var zero P
	nv, ok := extractFloat64(val)
	if !ok {
		return zero, fmt.Errorf("numeric field requires a numeric value, got %T", val)
	}
	switch op {
	case OpEQ:
		return f.eq(nv), nil
	case OpNEQ:
		return f.neq(nv), nil
	case OpLT:
		return f.lt(nv), nil
	case OpLTE:
		return f.lte(nv), nil
	case OpGT:
		return f.gt(nv), nil
	case OpGTE:
		return f.gte(nv), nil
	case OpHas:
		return zero, fmt.Errorf("operator ':' not supported for numeric fields")
	default:
		return zero, fmt.Errorf("operator %s not supported for numeric fields", op)
	}
}

// ---- value extraction helpers -----------------------------------------------

// extractString extracts a string from a filter Value.
// Accepts StringValue, TextValue, and WildcardValue ("*").
func extractString(v Value) (string, bool) {
	switch s := v.(type) {
	case *StringValue:
		return s.V, true
	case *TextValue:
		return s.V, true
	case *WildcardValue:
		return "*", true
	default:
		return "", false
	}
}

// extractFloat64 extracts a float64 from a filter Value.
// Only NumberValue is accepted.
func extractFloat64(v Value) (float64, bool) {
	n, ok := v.(*NumberValue)
	if !ok {
		return 0, false
	}
	return n.V, true
}
