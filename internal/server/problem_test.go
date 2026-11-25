package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/samber/do/v2"
	"github.com/samber/oops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/internal/codes"
	"github.com/liukeshao/echo-template/internal/i18n"
	"github.com/liukeshao/echo-template/internal/logging"
	"github.com/liukeshao/echo-template/internal/server"
	"github.com/liukeshao/echo-template/internal/testutil"
	"github.com/liukeshao/echo-template/pkg/rest"
	"github.com/liukeshao/echo-template/pkg/validator"
)

func TestProblemEncoder_AllClassificationPaths(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	tr := do.MustInvoke[i18n.Translator](env.Injector)
	encodeErr := server.NewProblemEncoder(tr)

	router := rest.NewRouter(rest.WithRouterErrorEncoder(encodeErr))
	router.Use(
		server.RequestID(),
		server.Locale(tr),
		rest.AccessLog(),
		rest.Recover(encodeErr),
		rest.BodyLimit(1<<20),
	)

	router.HandleFunc("GET /test/problem", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, rest.Problem{
			Status: http.StatusForbidden,
			Code:   "POLICY_DENIED",
			Detail: "Forbidden by policy.",
		})
	})
	router.HandleFunc("GET /test/problem-localized", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, rest.Problem{
			Status: http.StatusBadRequest,
			Code:   rest.CodePatchInvalid,
			Detail: "raw fallback",
			Params: map[string]string{"content_type": rest.ContentTypeMergePatch},
		})
	})
	router.HandleFunc("GET /test/oops-localized", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, oops.
			Code(codes.BookNotFound.String()).
			With(codes.CtxKeyParams, map[string]string{"book_id": "01TEST"}).
			Public("Book not found").
			Errorf("book not found"))
	})
	router.HandleFunc("GET /test/oops-internal", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, oops.
			Code(codes.BookInternalError.String()).
			With(codes.CtxKeyParams, map[string]string{"sql": "SELECT secret"}).
			Wrapf(errors.New("db password leak"), "querying book"))
	})
	router.HandleFunc("GET /test/validation", func(w http.ResponseWriter, r *http.Request) {
		var v validator.FieldViolations
		v.Add("title", validator.KeyRequired, map[string]string{"field": "title"})
		encodeErr(w, r, v.Err())
	})
	router.HandleFunc("GET /test/deadline", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, context.DeadlineExceeded)
	})
	router.HandleFunc("GET /test/canceled", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, context.Canceled)
	})
	router.HandleFunc("GET /test/unknown", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, errors.New("secret database connection string"))
	})
	router.HandleFunc("GET /test/panic", func(_ http.ResponseWriter, _ *http.Request) {
		panic("unexpected crash")
	})

	call := func(method, target, lang string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		if lang != "" {
			req.Header.Set("Accept-Language", lang)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// 1. Direct Problem -> fills type, title, instance, request_id
	rec := call(http.MethodGet, "/test/problem", "")
	p := testutil.AssertProblem(t, rec, http.StatusForbidden, "POLICY_DENIED")
	assert.Equal(t, "about:blank", p.Type)
	assert.Equal(t, "Forbidden", p.Title)
	assert.Equal(t, "Forbidden by policy.", p.Detail)
	assert.Equal(t, "/test/problem", p.Instance)
	assert.Equal(t, rec.Header().Get(server.HeaderRequestID), p.RequestID)

	// 1b. Problem whose Code exists in i18n [errors] (e.g. PATCH_INVALID) gets localized with Params
	rec = call(http.MethodGet, "/test/problem-localized", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusBadRequest, rest.CodePatchInvalid)
	assert.Contains(t, p.Detail, "PATCH 主体")
	assert.Contains(t, p.Detail, rest.ContentTypeMergePatch)

	// 2. oops with CtxKeyParams -> localized Chinese detail + exposed params
	rec = call(http.MethodGet, "/test/oops-localized", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusNotFound, "BOOK_NOT_FOUND")
	assert.Equal(t, "未找到 ID 为 01TEST 的书籍。", p.Detail)
	assert.Equal(t, map[string]string{"book_id": "01TEST"}, p.Params)

	// 3. oops 5xx -> localized generic message, internal error & params hidden
	rec = call(http.MethodGet, "/test/oops-internal", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusInternalServerError, "BOOK_INTERNAL_ERROR")
	assert.Equal(t, "处理书籍请求时发生内部错误。", p.Detail)
	assert.NotContains(t, p.Detail, "db password leak")
	assert.Empty(t, p.Params)

	// 4. *validator.FieldViolations -> localized 400 + errors
	rec = call(http.MethodGet, "/test/validation", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_INVALID")
	assert.Equal(t, "请求参数校验失败。", p.Detail)
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "title", p.Errors[0].Field)
	assert.Equal(t, "title 不能为空。", p.Errors[0].Message)

	// 5. context.DeadlineExceeded / context.Canceled (localized in zh-CN)
	p = testutil.AssertProblem(t, call(http.MethodGet, "/test/deadline", "zh-CN"), http.StatusGatewayTimeout, rest.CodeGatewayTimeout)
	assert.Equal(t, "请求处理超时。", p.Detail)
	p = testutil.AssertProblem(t, call(http.MethodGet, "/test/canceled", "zh-CN"), rest.StatusClientClosedRequest, rest.CodeClientClosedRequest)
	assert.Equal(t, "客户端在服务端响应前关闭了连接。", p.Detail)

	// 6. Unknown error -> 500 with details hidden
	p = testutil.AssertProblem(t, call(http.MethodGet, "/test/unknown", ""), http.StatusInternalServerError, rest.CodeInternalError)
	assert.NotContains(t, p.Detail, "secret database")

	// 7. Panic recovered -> 500 RFC 9457 Problem (preserving RequestID and negotiated locale)
	rec = call(http.MethodGet, "/test/panic", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusInternalServerError, rest.CodeInternalError)
	assert.NotContains(t, p.Detail, "unexpected crash")
	assert.Equal(t, "服务器内部错误。", p.Detail)
	assert.NotEmpty(t, p.RequestID)
	assert.Equal(t, rec.Header().Get(server.HeaderRequestID), p.RequestID)

	// 8. 404 & 405 from ServeMux converted to localized RFC 9457 Problem
	p = testutil.AssertProblem(t, call(http.MethodGet, "/does-not-exist", "zh-CN"), http.StatusNotFound, rest.CodeNotFound)
	assert.Equal(t, "/does-not-exist", p.Instance)
	assert.Equal(t, "请求的资源不存在。", p.Detail)

	rec = call(http.MethodPost, "/test/problem", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusMethodNotAllowed, rest.CodeMethodNotAllowed)
	assert.Equal(t, "该资源不支持当前 HTTP 方法。", p.Detail)
	assert.Contains(t, rec.Header().Get("Allow"), "GET")
}

func TestMiddlewares_RequestIDAndLocale(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	tr := do.MustInvoke[i18n.Translator](env.Injector)

	var gotRID, gotLocale string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRID = logging.RequestID(r.Context())
		gotLocale = i18n.Locale(r.Context(), tr.Default()).String()
		w.WriteHeader(http.StatusOK)
	})

	h := server.RequestID()(server.Locale(tr)(inner))
	req := httptest.NewRequest(http.MethodGet, "/?lang=zh-CN", nil)
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set(server.HeaderRequestID, "rid-999")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "rid-999", gotRID)
	assert.Equal(t, "rid-999", rec.Header().Get(server.HeaderRequestID))
	assert.Equal(t, "zh-CN", gotLocale)
}
