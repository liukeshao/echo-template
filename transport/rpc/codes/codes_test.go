package codes_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/transport/rpc/codes"
)

func TestCode_StringAndName(t *testing.T) {
	t.Parallel()
	for i := range uint32(17) {
		c := codes.Code(i)
		assert.NotEmpty(t, c.String())
		assert.NotEmpty(t, c.Name())
	}
	assert.Equal(t, "NOT_FOUND", codes.NotFound.Name())
	assert.Equal(t, "CANCELLED", codes.Canceled.Name())
	assert.Equal(t, "Code(99)", codes.Code(99).String())
	assert.Equal(t, "CODE(99)", codes.Code(99).Name())
}

func TestCode_UnmarshalJSON(t *testing.T) {
	t.Parallel()
	var c codes.Code
	require.NoError(t, json.Unmarshal([]byte(`"NOT_FOUND"`), &c))
	assert.Equal(t, codes.NotFound, c)

	require.NoError(t, json.Unmarshal([]byte(`5`), &c))
	assert.Equal(t, codes.NotFound, c)

	require.NoError(t, json.Unmarshal([]byte(`null`), &c))
	assert.Equal(t, codes.NotFound, c)

	assert.Error(t, json.Unmarshal([]byte(`99`), &c))
	assert.Error(t, json.Unmarshal([]byte(`"BOGUS"`), &c))

	var nilPtr *codes.Code
	assert.Error(t, nilPtr.UnmarshalJSON([]byte(`5`)))
}
