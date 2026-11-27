package httpkit_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/httperr"
	"github.com/liukeshao/echo-template/pkg/httpkit"
)

const samplePatchDoc = `{"id":"0195a3b0-7c4e-7000-8000-000000000001","title":"Old","author":"A","price":10,"meta":{"lang":"en","pages":100}}`

func TestApplyMergePatch(t *testing.T) {
	t.Parallel()
	body := []byte(`{"title":"New","price":null,"meta":{"pages":200}}`)
	out, err := httpkit.ApplyMergePatch([]byte(samplePatchDoc), body, httpkit.ContentTypeMergePatch)
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
		httpkit.ContentTypeMergePatch,
		httpkit.ContentTypeJSON,
		"",
		"application/merge-patch+json; charset=utf-8",
	} {
		out, err := httpkit.ApplyMergePatch([]byte(samplePatchDoc), []byte(`{"title":"Updated"}`), ct)
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
		_, err := httpkit.ApplyMergePatch([]byte(samplePatchDoc), []byte(`{}`), ct)
		require.Error(t, err, "contentType=%q", ct)

		p, ok := errors.AsType[httperr.Problem](err)
		require.True(t, ok)
		assert.Equal(t, 415, p.Status)
		assert.Empty(t, p.Detail, "transport errors must not hardcode user-facing Detail")
		mt, err := httperr.DecodeMetadata[httperr.PatchInfo](p)
		require.NoError(t, err)
		assert.Equal(t, ct, mt.ContentType)
		assert.Equal(t, httpkit.SupportedPatchMediaTypes, mt.Supported)
	}
}

func TestApplyMergePatch_InvalidJSONAndTrailingGarbage(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`{invalid json`,
		`{"title":"A"}{"title":"B"}`,
		``,
	} {
		_, err := httpkit.ApplyMergePatch([]byte(samplePatchDoc), []byte(body), httpkit.ContentTypeMergePatch)
		require.Error(t, err, "body=%q", body)

		p, ok := errors.AsType[httperr.Problem](err)
		require.True(t, ok)
		assert.Equal(t, 400, p.Status)
	}
}

func TestApplyMergePatchInto_SuccessAndTypeMismatch(t *testing.T) {
	t.Parallel()
	type target struct {
		Title string  `json:"title"`
		Price float64 `json:"price"`
	}

	var dst target
	require.NoError(t, httpkit.ApplyMergePatchInto([]byte(samplePatchDoc), []byte(`{"title":"Into"}`), "", &dst))
	assert.Equal(t, "Into", dst.Title)
	assert.InDelta(t, 10.0, dst.Price, 1e-9)

	var bad target
	err := httpkit.ApplyMergePatchInto([]byte(samplePatchDoc), []byte(`{"price":"not-a-number"}`), httpkit.ContentTypeMergePatch, &bad)
	require.Error(t, err)
	p, ok := errors.AsType[httperr.Problem](err)
	require.True(t, ok)
	assert.Equal(t, 400, p.Status)
	assert.Empty(t, p.Detail)
	mt, decErr := httperr.DecodeMetadata[httperr.PatchInfo](p)
	require.NoError(t, decErr)
	assert.Equal(t, httperr.PatchReasonSchemaMismatch, mt.Reason)
	assert.Equal(t, "price", mt.Field)

	err = httpkit.ApplyMergePatchInto([]byte(samplePatchDoc), []byte(`[{"op":"replace","path":"/title","value":"x"}]`), httpkit.ContentTypeMergePatch, &bad)
	require.Error(t, err)
	p, ok = errors.AsType[httperr.Problem](err)
	require.True(t, ok)
	assert.Equal(t, 400, p.Status)
}
