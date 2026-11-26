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

	"github.com/liukeshao/echo-template/internal/i18n"
	"github.com/liukeshao/echo-template/internal/server"
	"github.com/liukeshao/echo-template/internal/testutil"
	"github.com/liukeshao/echo-template/pkg/logging"
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
			Code:   rest.CodeForbidden,
			Metadata: rest.PreconditionInfo{
				Type:    "POLICY",
				Subject: "resource",
			},
		})
	})
	router.HandleFunc("GET /test/problem-localized", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, rest.Problem{
			Status: http.StatusBadRequest,
			Code:   rest.CodeBadRequest,
			Metadata: rest.MediaTypeInfo{
				ContentType: rest.ContentTypeMergePatch,
				Reason:      rest.PatchReasonInvalidJSON,
			},
		})
	})
	router.HandleFunc("GET /test/oops-localized", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, oops.
			In("book").
			Code(rest.CodeNotFound).
			User("internal-user-99").
			With("internal_sql", "SELECT * FROM books").
			With(rest.MetaKey, rest.ResourceInfo{
				ResourceType: "book",
				ResourceName: "01TEST",
			}).
			Public("SHOULD NEVER BE USED").
			Wrap(rest.CodeNotFound))
	})
	router.HandleFunc("GET /test/oops-retry-after", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, oops.
			In("weather").
			Code(rest.CodeTooManyRequests).
			With(rest.MetaKey, rest.RetryInfo{
				Service:    "weather",
				RetryAfter: "45",
			}).
			Wrap(rest.CodeTooManyRequests))
	})
	router.HandleFunc("GET /test/sentinel-direct", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, rest.CodeConflict)
	})
	router.HandleFunc("GET /test/oops-unregistered-ignores-public", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, oops.
			Code(rest.Code("UNREGISTERED_CUSTOM_CODE")).
			Public("leaked public text").
			New("unregistered"))
	})
	router.HandleFunc("GET /test/oops-internal", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, oops.
			In("book").
			Code(rest.CodeInternal).
			With(rest.MetaKey, rest.ResourceInfo{ResourceType: "book", ResourceName: "secret"}).
			With("sql", "SELECT secret").
			Join(rest.CodeInternal, errors.New("db password leak")))
	})
	router.HandleFunc("GET /test/oops-internal-canceled-ctx", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		cancel()
		r = r.WithContext(ctx)
		// Simulates a service wrapping ctx.Err() as CodeInternal: encoder must classify as 499, not 500.
		encodeErr(w, r, oops.
			In("book").
			Code(rest.CodeInternal).
			Join(rest.CodeInternal, ctx.Err()))
	})
	router.HandleFunc("GET /test/validation", func(w http.ResponseWriter, r *http.Request) {
		var v validator.FieldViolations
		v.Add("title", validator.CodeRequired, nil)
		v.Add("author", validator.CodeMaxLength, map[string]string{"max": "100"})
		encodeErr(w, r, v.Err())
	})
	router.HandleFunc("GET /test/maxbytes", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, &http.MaxBytesError{Limit: 1024})
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

	// 1. Direct Problem -> falls back from details.FORBIDDEN.POLICY to errors.FORBIDDEN + Content-Language header
	rec := call(http.MethodGet, "/test/problem", "")
	p := testutil.AssertProblem(t, rec, http.StatusForbidden, rest.CodeForbidden)
	assert.Equal(t, "about:blank", p.Type)
	assert.Equal(t, "Forbidden", p.Title)
	assert.Equal(t, "Access to this resource is forbidden.", p.Detail)
	assert.Equal(t, "en", rec.Header().Get("Content-Language"))
	assert.Equal(t, "/test/problem", p.Instance)
	assert.Equal(t, rec.Header().Get(server.HeaderRequestID), p.RequestID)
	precond := testutil.DecodeProblemMeta[rest.PreconditionInfo](t, p)
	assert.Equal(t, "POLICY", precond.Type)

	// 1b. Problem with MediaTypeInfo hits details.BAD_REQUEST.patch
	rec = call(http.MethodGet, "/test/problem-localized", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusBadRequest, rest.CodeBadRequest)
	assert.Equal(t, "zh-CN", rec.Header().Get("Content-Language"))
	assert.Contains(t, p.Detail, "PATCH 主体")
	assert.Contains(t, p.Detail, rest.ContentTypeMergePatch)
	mt := testutil.DecodeProblemMeta[rest.MediaTypeInfo](t, p)
	assert.Equal(t, rest.PatchReasonInvalidJSON, mt.Reason)

	// 2. oops with ResourceInfo -> hits details.NOT_FOUND.book; internal With/User/Public never leak
	rec = call(http.MethodGet, "/test/oops-localized", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusNotFound, rest.CodeNotFound)
	assert.Equal(t, "未找到 ID 为 01TEST 的书籍。", p.Detail)
	assert.Equal(t, "zh-CN", rec.Header().Get("Content-Language"))
	assert.NotContains(t, rec.Body.String(), "internal_sql")
	assert.NotContains(t, rec.Body.String(), "internal-user-99")
	assert.NotContains(t, rec.Body.String(), "SHOULD NEVER BE USED")
	resMeta := testutil.DecodeProblemMeta[rest.ResourceInfo](t, p)
	assert.Equal(t, rest.ResourceInfo{ResourceType: "book", ResourceName: "01TEST"}, resMeta)

	// 2b. 429 with RetryInfo -> sets standard HTTP Retry-After header + hits details.TOO_MANY_REQUESTS.weather
	rec = call(http.MethodGet, "/test/oops-retry-after", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusTooManyRequests, rest.CodeTooManyRequests)
	assert.Equal(t, "45", rec.Header().Get("Retry-After"))
	assert.Equal(t, "天气服务请求过于频繁，请稍后重试。", p.Detail)
	retryMeta := testutil.DecodeProblemMeta[rest.RetryInfo](t, p)
	assert.Equal(t, rest.RetryInfo{Service: "weather", RetryAfter: "45"}, retryMeta)

	// 2c. Direct sentinel rest.Code -> classified via rest.Classify and localized via errors.CONFLICT
	rec = call(http.MethodGet, "/test/sentinel-direct", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusConflict, rest.CodeConflict)
	assert.Equal(t, "请求与当前资源状态冲突。", p.Detail)

	// 2d. oops.Public is never surfaced even for unregistered codes, and 5xx falls back to localized INTERNAL_ERROR
	rec = call(http.MethodGet, "/test/oops-unregistered-ignores-public", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusInternalServerError, rest.Code("UNREGISTERED_CUSTOM_CODE"))
	assert.NotContains(t, p.Detail, "leaked public text")
	assert.Equal(t, "服务器内部错误。", p.Detail)

	// 3. oops 5xx -> localized generic message, internal error & metadata hidden
	rec = call(http.MethodGet, "/test/oops-internal", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusInternalServerError, rest.CodeInternal)
	assert.Equal(t, "服务器内部错误。", p.Detail)
	assert.NotContains(t, p.Detail, "db password leak")
	assert.Nil(t, p.Metadata)

	// 3b. oops 5xx wrapping canceled request ctx -> classified as 499 CLIENT_CLOSED_REQUEST (not 500!)
	rec = call(http.MethodGet, "/test/oops-internal-canceled-ctx", "zh-CN")
	p = testutil.AssertProblem(t, rec, rest.StatusClientClosedRequest, rest.CodeClientClosedRequest)
	assert.Equal(t, "客户端在服务端响应前关闭了连接。", p.Detail)

	// 4. *validator.FieldViolations -> 400 BAD_REQUEST + ValidationInfo + localized FieldErrors
	rec = call(http.MethodGet, "/test/validation", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusBadRequest, rest.CodeBadRequest)
	assert.Equal(t, "请求参数校验失败。", p.Detail)
	valMeta := testutil.DecodeProblemMeta[rest.ValidationInfo](t, p)
	assert.Equal(t, 2, valMeta.ErrorCount)
	require.Len(t, p.Errors, 2)
	assert.Equal(t, "title", p.Errors[0].Field)
	assert.Equal(t, validator.CodeRequired, p.Errors[0].Code)
	assert.Equal(t, "title 不能为空。", p.Errors[0].Message)
	assert.Nil(t, p.Errors[0].Metadata)
	assert.Equal(t, "author", p.Errors[1].Field)
	assert.Equal(t, validator.CodeMaxLength, p.Errors[1].Code)
	assert.Equal(t, "author 长度不能超过 100 个字符。", p.Errors[1].Message)
	assert.Equal(t, map[string]string{"max": "100"}, p.Errors[1].Metadata)

	// 5. *http.MaxBytesError -> 413 CONTENT_TOO_LARGE + RequestLimitInfo + localized detail with max_bytes
	rec = call(http.MethodGet, "/test/maxbytes", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusRequestEntityTooLarge, rest.CodeContentTooLarge)
	assert.Equal(t, "请求体大小超出允许上限（1024 字节）。", p.Detail)
	limitMeta := testutil.DecodeProblemMeta[rest.RequestLimitInfo](t, p)
	assert.Equal(t, int64(1024), limitMeta.MaxBytes)

	// 6. context.DeadlineExceeded / context.Canceled (localized in zh-CN)
	p = testutil.AssertProblem(t, call(http.MethodGet, "/test/deadline", "zh-CN"), http.StatusGatewayTimeout, rest.CodeGatewayTimeout)
	assert.Equal(t, "请求处理超时。", p.Detail)
	p = testutil.AssertProblem(t, call(http.MethodGet, "/test/canceled", "zh-CN"), rest.StatusClientClosedRequest, rest.CodeClientClosedRequest)
	assert.Equal(t, "客户端在服务端响应前关闭了连接。", p.Detail)

	// 7. Unknown error -> 500 with details hidden
	p = testutil.AssertProblem(t, call(http.MethodGet, "/test/unknown", ""), http.StatusInternalServerError, rest.CodeInternal)
	assert.NotContains(t, p.Detail, "secret database")

	// 8. Panic recovered -> 500 RFC 9457 Problem (preserving RequestID and negotiated locale)
	rec = call(http.MethodGet, "/test/panic", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusInternalServerError, rest.CodeInternal)
	assert.NotContains(t, p.Detail, "unexpected crash")
	assert.Equal(t, "服务器内部错误。", p.Detail)
	assert.NotEmpty(t, p.RequestID)
	assert.Equal(t, rec.Header().Get(server.HeaderRequestID), p.RequestID)

	// 9. 404 & 405 from ServeMux converted to localized RFC 9457 Problem with ResourceInfo / MethodInfo
	p = testutil.AssertProblem(t, call(http.MethodGet, "/does-not-exist", "zh-CN"), http.StatusNotFound, rest.CodeNotFound)
	assert.Equal(t, "/does-not-exist", p.Instance)
	assert.Equal(t, "请求的资源不存在。", p.Detail)
	nfMeta := testutil.DecodeProblemMeta[rest.ResourceInfo](t, p)
	assert.Equal(t, rest.ResourceInfo{ResourceType: "route", ResourceName: "/does-not-exist"}, nfMeta)

	rec = call(http.MethodPost, "/test/problem", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusMethodNotAllowed, rest.CodeMethodNotAllowed)
	assert.Equal(t, "该资源不支持当前 HTTP 方法。", p.Detail)
	assert.Contains(t, rec.Header().Get("Allow"), "GET")
	methodMeta := testutil.DecodeProblemMeta[rest.MethodInfo](t, p)
	assert.Equal(t, http.MethodPost, methodMeta.Method)
	assert.Equal(t, "/test/problem", methodMeta.Path)
	assert.Contains(t, methodMeta.Allowed, http.MethodGet)
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
