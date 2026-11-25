package handler_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/internal/codes"
	"github.com/liukeshao/echo-template/internal/testutil"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/rest"
)

func TestBookHandler_PatchErrorsReturnProperHTTPStatus(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	authResp := env.MustAuthUser("patcher", "patcher@example.com", "password123")

	createRec := env.DoRequest(http.MethodPost, "/api/v1/books",
		testutil.WithJSON(map[string]any{"title": "Original", "author": "Author", "price": 10}),
		testutil.WithBearer(authResp.AccessToken),
	)
	require.Equal(t, http.StatusCreated, createRec.Code)
	created := testutil.DecodeJSON[api.Book](t, createRec)

	// Wrong type for price -> 400 PATCH_INVALID with MediaTypeInfo (not 500).
	patchRec := env.DoRequest(http.MethodPatch, "/api/v1/books/"+created.ID,
		testutil.WithBody(rest.ContentTypeMergePatch, `{"price":"not-a-number"}`),
		testutil.WithBearer(authResp.AccessToken),
	)
	p := testutil.AssertProblem(t, patchRec, http.StatusBadRequest, codes.PatchInvalid.String())
	mt := testutil.DecodeProblemMeta[rest.MediaTypeInfo](t, p)
	assert.Equal(t, "schema_mismatch", mt.Reason)
	assert.Equal(t, "price", mt.Field)

	// Unsupported PATCH Content-Type -> 415 PATCH_UNSUPPORTED_MEDIA_TYPE with MediaTypeInfo.
	unsupportedRec := env.DoRequest(http.MethodPatch, "/api/v1/books/"+created.ID,
		testutil.WithBody("text/plain", `title=foo`),
		testutil.WithBearer(authResp.AccessToken),
	)
	p = testutil.AssertProblem(t, unsupportedRec, http.StatusUnsupportedMediaType, codes.PatchUnsupportedMediaType.String())
	mt = testutil.DecodeProblemMeta[rest.MediaTypeInfo](t, p)
	assert.Equal(t, "text/plain", mt.ContentType)
	assert.Equal(t, rest.SupportedPatchMediaTypes, mt.Supported)
}

func TestBookHandler_CRUDAndMergePatch(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	tok := env.MustAuthUser("bookuser", "bookuser@example.com", "password123")
	auth := testutil.WithBearer(tok.AccessToken)

	// Create
	rec := env.DoRequest(http.MethodPost, "/api/v1/books",
		testutil.WithJSON(map[string]any{"title": "Go", "author": "Alan", "description": "Desc", "price": 20}),
		auth,
	)
	require.Equal(t, http.StatusCreated, rec.Code)
	b := testutil.DecodeJSON[api.Book](t, rec)

	// Get
	rec = env.DoRequest(http.MethodGet, "/api/v1/books/"+b.ID, auth)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Go", testutil.DecodeJSON[api.Book](t, rec).Title)

	// Get invalid UUID -> 400 VALIDATION_INVALID
	rec = env.DoRequest(http.MethodGet, "/api/v1/books/not-a-uuid", auth)
	testutil.AssertProblem(t, rec, http.StatusBadRequest, codes.ValidationInvalid.String())

	// List invalid integer query -> 400 VALIDATION_INVALID
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?page=abc", auth)
	p := testutil.AssertProblem(t, rec, http.StatusBadRequest, codes.ValidationInvalid.String())
	require.Len(t, p.Errors, 1)
	assert.Contains(t, p.Errors[0].Message, "valid integer")

	// List invalid float query -> 400 VALIDATION_INVALID
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?min_price=not-a-number", auth)
	p = testutil.AssertProblem(t, rec, http.StatusBadRequest, codes.ValidationInvalid.String())
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "min_price", p.Errors[0].Field)
	assert.Contains(t, p.Errors[0].Message, "valid number")

	// List negative min_price -> 400 VALIDATION_INVALID
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?min_price=-1", auth)
	testutil.AssertProblem(t, rec, http.StatusBadRequest, codes.ValidationInvalid.String())

	// List with explicit typed query fields
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?page=1&page_size=10&order_by=title&title=go&min_price=10&max_price=30", auth)
	require.Equal(t, http.StatusOK, rec.Code)
	list := testutil.DecodeJSON[api.BookList](t, rec)
	assert.Equal(t, int32(1), list.TotalSize)
	require.Len(t, list.Items, 1)
	assert.Equal(t, b.ID, list.Items[0].ID)

	// Replace (PUT)
	rec = env.DoRequest(http.MethodPut, "/api/v1/books/"+b.ID,
		testutil.WithJSON(map[string]any{"title": "Go 2", "author": "Alan", "price": 30}),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	got := testutil.DecodeJSON[api.Book](t, rec)
	assert.Equal(t, "Go 2", got.Title)
	assert.Empty(t, got.Description)

	// PATCH RFC 7386 Merge Patch + attempt to mutate read-only id (must be ignored)
	rec = env.DoRequest(http.MethodPatch, "/api/v1/books/"+b.ID,
		testutil.WithBody(rest.ContentTypeMergePatch, `{"id":"FAKE","title":"Merged","price":null}`),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	got = testutil.DecodeJSON[api.Book](t, rec)
	assert.Equal(t, b.ID, got.ID)
	assert.Equal(t, "Merged", got.Title)
	assert.Nil(t, got.Price)

	// PATCH with application/json compatibility
	rec = env.DoRequest(http.MethodPatch, "/api/v1/books/"+b.ID,
		testutil.WithBody(rest.ContentTypeJSON, `{"title":"JSONCompat","price":15.5}`),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	got = testutil.DecodeJSON[api.Book](t, rec)
	assert.Equal(t, "JSONCompat", got.Title)
	require.NotNil(t, got.Price)
	assert.InDelta(t, 15.5, *got.Price, 1e-9)

	// Delete -> 204
	rec = env.DoRequest(http.MethodDelete, "/api/v1/books/"+b.ID, auth)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
