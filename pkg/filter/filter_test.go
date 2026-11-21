package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_NegativeNumbers(t *testing.T) {
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

func TestParse_InvalidNegativeValue(t *testing.T) {
	_, err := Parse("price = -abc")
	require.Error(t, err)
}

func TestParse_PrecedenceAndNot(t *testing.T) {
	// AIP-160: OR binds tighter than AND.
	expr, err := Parse(`a = 1 AND b = 2 OR -c = 3`)
	require.NoError(t, err)

	andExpr, ok := expr.(*AndExpr)
	require.True(t, ok)
	_, ok = andExpr.Right.(*OrExpr)
	require.True(t, ok)
}
