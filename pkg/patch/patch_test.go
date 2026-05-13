package patch_test

import (
	"encoding/json"
	"testing"

	"github.com/samber/oops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/patch"
)

const sample = `{"id":"01H","title":"Old","author":"A","price":10}`

func TestApply_JSONPatch(t *testing.T) {
	body := []byte(`[
		{"op":"replace","path":"/title","value":"New"},
		{"op":"replace","path":"/price","value":12.5}
	]`)
	out, err := patch.Apply([]byte(sample), body, patch.ContentTypeJSONPatch)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "New", got["title"])
	assert.InDelta(t, 12.5, got["price"], 1e-9)
	assert.Equal(t, "A", got["author"])
}

func TestApply_MergePatch(t *testing.T) {
	body := []byte(`{"title":"New","price":null}`)
	out, err := patch.Apply([]byte(sample), body, patch.ContentTypeMergePatch)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "New", got["title"])
	_, hasPrice := got["price"]
	assert.False(t, hasPrice, "price should be removed by null merge patch")
}

func TestApply_ShorthandPatch(t *testing.T) {
	body := []byte(`title: "New", price: 12.5`)
	out, err := patch.Apply([]byte(sample), body, patch.ContentTypeShorthandPatch)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "New", got["title"])
	assert.InDelta(t, 12.5, got["price"], 1e-9)
	assert.Equal(t, "A", got["author"], "untouched fields must survive shorthand patch")
}

func TestApply_ShorthandPatch_Braced(t *testing.T) {
	body := []byte(`{title: "Braced", author: "B"}`)
	out, err := patch.Apply([]byte(sample), body, patch.ContentTypeShorthandPatch)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "Braced", got["title"])
	assert.Equal(t, "B", got["author"])
}

func TestApply_DefaultsToMergePatch(t *testing.T) {
	body := []byte(`{"title":"Default"}`)
	out, err := patch.Apply([]byte(sample), body, "")
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "Default", got["title"])
}

func TestApply_UnsupportedMediaType(t *testing.T) {
	_, err := patch.Apply([]byte(sample), []byte(`{}`), "application/xml")
	require.Error(t, err)

	var oe oops.OopsError
	require.ErrorAs(t, err, &oe)
	assert.Equal(t, codes.PatchUnsupportedMediaType.String(), oe.Code())
}

func TestApply_InvalidJSONPatch(t *testing.T) {
	_, err := patch.Apply([]byte(sample), []byte(`not a patch`), patch.ContentTypeJSONPatch)
	require.Error(t, err)

	var oe oops.OopsError
	require.ErrorAs(t, err, &oe)
	assert.Equal(t, codes.PatchInvalid.String(), oe.Code())
}

func TestApply_ContentTypeWithParameters(t *testing.T) {
	body := []byte(`{"title":"Charset"}`)
	out, err := patch.Apply([]byte(sample), body, "application/merge-patch+json; charset=utf-8")
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "Charset", got["title"])
}
