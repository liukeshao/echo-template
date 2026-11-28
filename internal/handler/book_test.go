package handler_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/internal/testkit"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/httperr"
	"github.com/liukeshao/echo-template/pkg/httpkit"
)

func TestBookHandler_PatchErrorsReturnProperHTTPStatus(t *testing.T) {
	t.Parallel()
	env := testkit.New(t)
	authResp := env.MustAuthUser("patcher", "patcher@example.com", "password123")

	createRec := env.DoRequest(http.MethodPost, "/api/v1/books",
		testkit.WithJSON(map[string]any{"title": "Original", "author": "Author", "price": 10}),
		testkit.WithBearer(authResp.AccessToken),
	)
	require.Equal(t, http.StatusCreated, createRec.Code)
	created := testkit.DecodeJSON[api.Book](t, createRec)

	// Wrong type for price -> 400 with PatchInfo (not 500).
	patchRec := env.DoRequest(http.MethodPatch, "/api/v1/books/"+created.ID,
		testkit.WithBody(httpkit.ContentTypeMergePatch, `{"price":"not-a-number"}`),
		testkit.WithBearer(authResp.AccessToken),
	)
	p := testkit.AssertProblem(t, patchRec, http.StatusBadRequest)
	mt := testkit.DecodeProblemMeta[httperr.PatchInfo](t, p)
	assert.Equal(t, httperr.PatchSchemaMismatch, mt.Reason)
	assert.Equal(t, "price", mt.Field)

	// Unsupported PATCH Content-Type -> 415 with PatchInfo.
	unsupportedRec := env.DoRequest(http.MethodPatch, "/api/v1/books/"+created.ID,
		testkit.WithBody("text/plain", `title=foo`),
		testkit.WithBearer(authResp.AccessToken),
	)
	p = testkit.AssertProblem(t, unsupportedRec, http.StatusUnsupportedMediaType)
	mt = testkit.DecodeProblemMeta[httperr.PatchInfo](t, p)
	assert.Equal(t, "text/plain", mt.ContentType)
	assert.Equal(t, httpkit.SupportedPatchMediaTypes, mt.Supported)
}

func TestBookHandler_CRUDAndMergePatch(t *testing.T) {
	t.Parallel()
	env := testkit.New(t)
	tok := env.MustAuthUser("bookuser", "bookuser@example.com", "password123")
	auth := testkit.WithBearer(tok.AccessToken)

	// Create
	rec := env.DoRequest(http.MethodPost, "/api/v1/books",
		testkit.WithJSON(map[string]any{"title": "Go", "author": "Alan", "description": "Desc", "price": 20}),
		auth,
	)
	require.Equal(t, http.StatusCreated, rec.Code)
	b := testkit.DecodeJSON[api.Book](t, rec)

	// Get
	rec = env.DoRequest(http.MethodGet, "/api/v1/books/"+b.ID, auth)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Go", testkit.DecodeJSON[api.Book](t, rec).Title)

	// Get invalid UUID -> 400 + ValidationInfo
	rec = env.DoRequest(http.MethodGet, "/api/v1/books/not-a-uuid", auth)
	testkit.AssertProblem(t, rec, http.StatusBadRequest)

	// List invalid integer query -> 400 + FieldError
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?page=abc", auth)
	p := testkit.AssertProblem(t, rec, http.StatusBadRequest)
	require.Len(t, p.Errors, 1)
	assert.Contains(t, p.Errors[0].Message, "valid integer")

	// List invalid float query -> 400 + FieldError
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?min_price=not-a-number", auth)
	p = testkit.AssertProblem(t, rec, http.StatusBadRequest)
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "min_price", p.Errors[0].Field)
	assert.Contains(t, p.Errors[0].Message, "valid number")

	// List negative min_price -> 400
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?min_price=-1", auth)
	testkit.AssertProblem(t, rec, http.StatusBadRequest)

	// List with explicit typed query fields
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?page=1&page_size=10&order_by=title&title=go&min_price=10&max_price=30", auth)
	require.Equal(t, http.StatusOK, rec.Code)
	list := testkit.DecodeJSON[api.BookList](t, rec)
	assert.Equal(t, int32(1), list.TotalSize)
	require.Len(t, list.Items, 1)
	assert.Equal(t, b.ID, list.Items[0].ID)

	// Replace (PUT)
	rec = env.DoRequest(http.MethodPut, "/api/v1/books/"+b.ID,
		testkit.WithJSON(map[string]any{"title": "Go 2", "author": "Alan", "price": 30}),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	got := testkit.DecodeJSON[api.Book](t, rec)
	assert.Equal(t, "Go 2", got.Title)
	assert.Empty(t, got.Description)

	// PATCH RFC 7386 Merge Patch + attempt to mutate read-only id (must be ignored)
	rec = env.DoRequest(http.MethodPatch, "/api/v1/books/"+b.ID,
		testkit.WithBody(httpkit.ContentTypeMergePatch, `{"id":"FAKE","title":"Merged","price":null}`),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	got = testkit.DecodeJSON[api.Book](t, rec)
	assert.Equal(t, b.ID, got.ID)
	assert.Equal(t, "Merged", got.Title)
	assert.Nil(t, got.Price)

	// PATCH with application/json compatibility
	rec = env.DoRequest(http.MethodPatch, "/api/v1/books/"+b.ID,
		testkit.WithBody(httpkit.ContentTypeJSON, `{"title":"JSONCompat","price":15.5}`),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	got = testkit.DecodeJSON[api.Book](t, rec)
	assert.Equal(t, "JSONCompat", got.Title)
	require.NotNil(t, got.Price)
	assert.InDelta(t, 15.5, *got.Price, 1e-9)

	// Delete -> 204
	rec = env.DoRequest(http.MethodDelete, "/api/v1/books/"+b.ID, auth)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
