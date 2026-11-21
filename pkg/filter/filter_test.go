package filter

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_Empty(t *testing.T) {
	t.Parallel()
	expr, err := Parse("   \t\n ")
	require.NoError(t, err)
	assert.Nil(t, expr)
}

func TestParse_NegativeNumbers(t *testing.T) {
	t.Parallel()
	expr, err := Parse("price < -10.5 AND author = 'Bob'")
	require.NoError(t, err)

	andExpr, ok := expr.(*AndExpr)
	require.True(t, ok)

	left, ok := andExpr.Left.(*RestrictionExpr)
	require.True(t, ok)
	assert.Equal(t, "price", left.Field)
	assert.Equal(t, OpLT, left.Op)

	nv, ok := left.Value.(*NumberValue)
	require.True(t, ok)
	assert.InDelta(t, -10.5, nv.V, 1e-9)
}

func TestParse_PrecedenceAndNot(t *testing.T) {
	t.Parallel()
	// AIP-160: OR binds tighter than AND.
	expr, err := Parse(`a = 1 AND b = 2 OR -c = 3`)
	require.NoError(t, err)

	andExpr, ok := expr.(*AndExpr)
	require.True(t, ok)
	orExpr, ok := andExpr.Right.(*OrExpr)
	require.True(t, ok)
	_, ok = orExpr.Right.(*NotExpr)
	require.True(t, ok)
}

func TestParse_LiteralsEscapesDottedAndParens(t *testing.T) {
	t.Parallel()
	expr, err := Parse(`NOT (author.name : "A\"B\\C\n\t\x" AND active = true AND disabled = false AND tag = * AND rank >= 1.5e2 AND status = ACTIVE)`)
	require.NoError(t, err)
	notExpr, ok := expr.(*NotExpr)
	require.True(t, ok)
	require.NotNil(t, notExpr.Operand)
}

func TestParse_AllComparators(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in string
		op Op
	}{
		{"x = 1", OpEQ},
		{"x != 1", OpNEQ},
		{"x < 1", OpLT},
		{"x <= 1", OpLTE},
		{"x > 1", OpGT},
		{"x >= 1", OpGTE},
		{"x : \"a\"", OpHas},
	}
	for _, tc := range cases {
		expr, err := Parse(tc.in)
		require.NoError(t, err, tc.in)
		r, ok := expr.(*RestrictionExpr)
		require.True(t, ok)
		assert.Equal(t, tc.op, r.Op)
		assert.NotEmpty(t, r.Op.String())
	}
}

func TestParse_Errors(t *testing.T) {
	t.Parallel()
	badInputs := []string{
		"price = -abc",
		"title ! \"a\"",
		"title = \"unterminated",
		"title = @invalid",
		"(title = \"a\"",
		"AND title = \"a\"",
		"title. = \"a\"",
		"title \"a\"",
		"title = )",
		"title = \"a\" trailing",
	}
	for _, in := range badInputs {
		_, err := Parse(in)
		assert.Error(t, err, "expected error for %q", in)
	}
}

func TestFilterTranslator(t *testing.T) {
	t.Parallel()

	mkStr := func(op string) func(string) string {
		return func(v string) string { return fmt.Sprintf("%s(%s)", op, v) }
	}
	mkNum := func(op string) func(float64) string {
		return func(v float64) string { return fmt.Sprintf("%s(%.1f)", op, v) }
	}

	tr := NewTranslator(
		func(ps ...string) string { return "(" + strings.Join(ps, " AND ") + ")" },
		func(ps ...string) string { return "(" + strings.Join(ps, " OR ") + ")" },
		func(p string) string { return "NOT(" + p + ")" },
	).
		String("title",
			mkStr("eq"), mkStr("neq"),
			mkStr("lt"), mkStr("lte"), mkStr("gt"), mkStr("gte"),
			mkStr("contains"), mkStr("prefix"), mkStr("suffix"),
		).
		Float64("price",
			mkNum("eq"), mkNum("neq"),
			mkNum("lt"), mkNum("lte"), mkNum("gt"), mkNum("gte"),
		)

	// Nil expr returns zero value.
	got, err := tr.Translate(nil)
	require.NoError(t, err)
	assert.Empty(t, got)

	cases := []struct {
		in   string
		want string
	}{
		{`title = "Go"`, "eq(Go)"},
		{`title = "Go*"`, "prefix(Go)"},
		{`title = "*Go"`, "suffix(Go)"},
		{`title = "*Go*"`, "contains(Go)"},
		{`title = "G*o"`, "contains(Go)"},
		{`title = *`, "contains()"},
		{`title != "Java"`, "neq(Java)"},
		{`title < "B"`, "lt(B)"},
		{`title <= "B"`, "lte(B)"},
		{`title > "B"`, "gt(B)"},
		{`title >= "B"`, "gte(B)"},
		{`title : "Go"`, "contains(Go)"},
		{`title = BareWord`, "eq(BareWord)"},
		{`price = 10`, "eq(10.0)"},
		{`price != 10`, "neq(10.0)"},
		{`price < 10`, "lt(10.0)"},
		{`price <= 10`, "lte(10.0)"},
		{`price > 10`, "gt(10.0)"},
		{`price >= 10`, "gte(10.0)"},
		{`NOT title = "Go" AND price < 50 OR price > 100`, "(NOT(eq(Go)) AND (lt(50.0) OR gt(100.0)))"},
	}
	for _, tc := range cases {
		expr, err := Parse(tc.in)
		require.NoError(t, err, tc.in)
		got, err := tr.Translate(expr)
		require.NoError(t, err, tc.in)
		assert.Equal(t, tc.want, got, tc.in)
	}

	// Error cases: unknown field, type mismatch, ':' on numeric field.
	for _, bad := range []string{
		`unknown = "x"`,
		`title = 123`,
		`price = "ten"`,
		`price : 10`,
	} {
		expr, err := Parse(bad)
		require.NoError(t, err, bad)
		_, err = tr.Translate(expr)
		assert.Error(t, err, "expected translate error for %q", bad)
	}
}
