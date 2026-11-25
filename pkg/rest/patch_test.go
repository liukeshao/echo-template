package rest_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/rest"
)

const samplePatchDoc = `{"id":"0195a3b0-7c4e-7000-8000-000000000001","title":"Old","author":"A","price":10,"meta":{"lang":"en","pages":100}}`

func TestApplyMergePatch(t *testing.T) {
	t.Parallel()
	body := []byte(`{"title":"New","price":null,"meta":{"pages":200}}`)
	out, err := rest.ApplyMergePatch([]byte(samplePatchDoc), body, rest.ContentTypeMergePatch)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "New", got["title"])
	assert.Equal(t, "A", got["author"])
	_, hasPrice := got["price"]
	assert.False(t, hasPrice, "price should be removed by null merge patch")

	meta, ok := got["meta"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "en", meta["lang"])
	assert.InDelta(t, 200.0, meta["pages"], 1e-9)
}

func TestApplyMergePatch_AcceptsJSONEmptyAndCharsetParams(t *testing.T) {
	t.Parallel()
	for _, ct := range []string{
		rest.ContentTypeMergePatch,
		rest.ContentTypeJSON,
		"",
		"application/merge-patch+json; charset=utf-8",
	} {
		out, err := rest.ApplyMergePatch([]byte(samplePatchDoc), []byte(`{"title":"Updated"}`), ct)
		require.NoError(t, err, "contentType=%q", ct)
		var got map[string]any
		require.NoError(t, json.Unmarshal(out, &got))
		assert.Equal(t, "Updated", got["title"])
	}
}

func TestApplyMergePatch_UnsupportedMediaTypes(t *testing.T) {
	t.Parallel()
	for _, ct := range []string{
		"application/xml",
		"application/json-patch+json",
		"application/merge-patch+shorthand",
		";;;bad-media-type",
	} {
		_, err := rest.ApplyMergePatch([]byte(samplePatchDoc), []byte(`{}`), ct)
		require.Error(t, err, "contentType=%q", ct)

		p, ok := errors.AsType[rest.Problem](err)
		require.True(t, ok)
		assert.Equal(t, 415, p.Status)
		assert.Equal(t, rest.CodePatchUnsupportedMediaType, p.Code)
	}
}

func TestApplyMergePatch_InvalidJSONAndTrailingGarbage(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`{invalid json`,
		`{"title":"A"}{"title":"B"}`,
		``,
	} {
		_, err := rest.ApplyMergePatch([]byte(samplePatchDoc), []byte(body), rest.ContentTypeMergePatch)
		require.Error(t, err, "body=%q", body)

		p, ok := errors.AsType[rest.Problem](err)
		require.True(t, ok)
		assert.Equal(t, 400, p.Status)
		assert.Equal(t, rest.CodePatchInvalid, p.Code)
	}
}

func TestApplyMergePatchInto_SuccessAndTypeMismatch(t *testing.T) {
	t.Parallel()
	type target struct {
		Title string  `json:"title"`
		Price float64 `json:"price"`
	}

	var dst target
	require.NoError(t, rest.ApplyMergePatchInto([]byte(samplePatchDoc), []byte(`{"title":"Into"}`), "", &dst))
	assert.Equal(t, "Into", dst.Title)
	assert.InDelta(t, 10.0, dst.Price, 1e-9)

	var bad target
	err := rest.ApplyMergePatchInto([]byte(samplePatchDoc), []byte(`{"price":"not-a-number"}`), rest.ContentTypeMergePatch, &bad)
	require.Error(t, err)
	p, ok := errors.AsType[rest.Problem](err)
	require.True(t, ok)
	assert.Equal(t, rest.CodePatchInvalid, p.Code)

	err = rest.ApplyMergePatchInto([]byte(samplePatchDoc), []byte(`[{"op":"replace","path":"/title","value":"x"}]`), rest.ContentTypeMergePatch, &bad)
	require.Error(t, err)
	p, ok = errors.AsType[rest.Problem](err)
	require.True(t, ok)
	assert.Equal(t, rest.CodePatchInvalid, p.Code)
}
