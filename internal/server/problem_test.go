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
	"github.com/liukeshao/echo-template/internal/testkit"
	"github.com/liukeshao/echo-template/pkg/httperr"
	"github.com/liukeshao/echo-template/pkg/httpkit"
	"github.com/liukeshao/echo-template/pkg/logging"
	"github.com/liukeshao/echo-template/pkg/validator"
)

func TestProblemEncoder_AllClassificationPaths(t *testing.T) {
	t.Parallel()
	env := testkit.New(t)
	tr := do.MustInvoke[i18n.Translator](env.Injector)
	encodeErr := server.NewProblemEncoder(tr)

	router := httpkit.NewRouter()
	router.SetErrorEncoder(encodeErr)
	router.Use(
		httpkit.RequestID(),
		server.Locale(tr),
		httpkit.AccessLog(),
		httpkit.Recover(encodeErr),
		httpkit.BodyLimit(1<<20),
	)

	router.HandleFunc("GET /test/problem", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, httperr.Problem{
			Status: http.StatusForbidden,
			Metadata: httperr.PreconditionInfo{
				Type:    "POLICY",
				Subject: "resource",
			},
		})
	})
	router.HandleFunc("GET /test/problem-localized", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, httperr.Problem{
			Status: http.StatusBadRequest,
			Metadata: httperr.PatchInfo{
				ContentType: httpkit.ContentTypeMergePatch,
				Reason:      httperr.PatchInvalidJSON,
			},
		})
	})
	router.HandleFunc("GET /test/problem-prefilled", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, httperr.Problem{
			Status: http.StatusForbidden,
			Detail: "raw english detail that must be localized",
		})
	})
	router.HandleFunc("GET /test/oops-localized", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, oops.
			In("book").
			User("internal-user-99").
			With("internal_sql", "SELECT * FROM books").
			With(httperr.MetaKey, httperr.ResourceInfo{
				ResourceType: "book",
				ResourceName: "01TEST",
			}).
			Public("SHOULD NEVER BE USED").
			Wrap(httperr.NotFound))
	})
	router.HandleFunc("GET /test/oops-retry-after", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, oops.
			In("weather").
			With(httperr.MetaKey, httperr.RetryInfo{
				Service:    "weather",
				RetryAfter: "45",
			}).
			Wrap(httperr.TooManyRequests))
	})
	router.HandleFunc("GET /test/sentinel-direct", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, httperr.Conflict)
	})
	router.HandleFunc("GET /test/sentinel-413", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, httperr.ContentTooLarge)
	})
	router.HandleFunc("GET /test/sentinel-415", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, httperr.UnsupportedMediaType)
	})
	router.HandleFunc("GET /test/oops-unregistered-ignores-public", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, oops.
			Public("leaked public text").
			Wrap(httperr.Status(599)))
	})
	router.HandleFunc("GET /test/oops-internal", func(w http.ResponseWriter, r *http.Request) {
		encodeErr(w, r, oops.
			In("book").
			With(httperr.MetaKey, httperr.ResourceInfo{ResourceType: "book", ResourceName: "secret"}).
			With("sql", "SELECT secret").
			Join(httperr.Internal, errors.New("db password leak")))
	})
	router.HandleFunc("GET /test/oops-upstream-timeout", func(w http.ResponseWriter, r *http.Request) {
		// Upstream client timeout explicitly mapped to 503 while caller context is still alive must stay 503 (not 504).
		encodeErr(w, r, oops.
			In("weather").
			Join(httperr.ServiceUnavailable, context.DeadlineExceeded))
	})
	router.HandleFunc("GET /test/oops-internal-canceled-ctx", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		cancel()
		r = r.WithContext(ctx)
		// Simulates a service wrapping ctx.Err() as httperr.Internal: encoder must classify as 499, not 500.
		encodeErr(w, r, oops.
			In("book").
			Join(httperr.Internal, ctx.Err()))
	})
	router.HandleFunc("GET /test/validation", func(w http.ResponseWriter, r *http.Request) {
		var v validator.FieldViolations
		v.Add("title", validator.RuleRequired)
		v.Add("author", validator.RuleMaxLength, validator.P("max", "100"))
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

	// 1. Direct Problem -> falls back from details.403.POLICY to errors.403 + Content-Language header
	rec := call(http.MethodGet, "/test/problem", "")
	p := testkit.AssertProblem(t, rec, http.StatusForbidden)
	assert.Equal(t, "about:blank", p.Type)
	assert.Equal(t, "Forbidden", p.Title)
	assert.Equal(t, "Access to this resource is forbidden.", p.Detail)
	assert.Equal(t, "en", rec.Header().Get("Content-Language"))
	assert.Equal(t, "/test/problem", p.Instance)
	assert.Equal(t, rec.Header().Get(httpkit.HeaderRequestID), p.RequestID)
	precond := testkit.DecodeProblemMeta[httperr.PreconditionInfo](t, p)
	assert.Equal(t, "POLICY", precond.Type)

	// 1b. Problem with PatchInfo hits details.400.patch
	rec = call(http.MethodGet, "/test/problem-localized", "zh-CN")
	p = testkit.AssertProblem(t, rec, http.StatusBadRequest)
	assert.Equal(t, "zh-CN", rec.Header().Get("Content-Language"))
	assert.Contains(t, p.Detail, "PATCH 主体")
	assert.Contains(t, p.Detail, httpkit.ContentTypeMergePatch)
	mt := testkit.DecodeProblemMeta[httperr.PatchInfo](t, p)
	assert.Equal(t, httperr.PatchInvalidJSON, mt.Reason)

	// 1c. Pre-filled Problem.Detail does not bypass terminal i18n
	rec = call(http.MethodGet, "/test/problem-prefilled", "zh-CN")
	p = testkit.AssertProblem(t, rec, http.StatusForbidden)
	assert.Equal(t, "无权访问该资源。", p.Detail)

	// 2. oops with ResourceInfo -> hits details.404.book; internal With/User/Public never leak
	rec = call(http.MethodGet, "/test/oops-localized", "zh-CN")
	p = testkit.AssertProblem(t, rec, http.StatusNotFound)
	assert.Equal(t, "未找到 ID 为 01TEST 的书籍。", p.Detail)
	assert.Equal(t, "zh-CN", rec.Header().Get("Content-Language"))
	assert.NotContains(t, rec.Body.String(), "internal_sql")
	assert.NotContains(t, rec.Body.String(), "internal-user-99")
	assert.NotContains(t, rec.Body.String(), "SHOULD NEVER BE USED")
	resMeta := testkit.DecodeProblemMeta[httperr.ResourceInfo](t, p)
	assert.Equal(t, httperr.ResourceInfo{ResourceType: "book", ResourceName: "01TEST"}, resMeta)

	// 2b. 429 with RetryInfo -> sets standard HTTP Retry-After header + hits details.429.weather
	rec = call(http.MethodGet, "/test/oops-retry-after", "zh-CN")
	p = testkit.AssertProblem(t, rec, http.StatusTooManyRequests)
	assert.Equal(t, "45", rec.Header().Get("Retry-After"))
	assert.Equal(t, "天气服务请求过于频繁，请稍后重试。", p.Detail)
	retryMeta := testkit.DecodeProblemMeta[httperr.RetryInfo](t, p)
	assert.Equal(t, httperr.RetryInfo{Service: "weather", RetryAfter: "45"}, retryMeta)

	// 2c. Direct sentinel httperr.Status -> classified via httperr.Classify and localized via clean placeholder-free [errors]
	rec = call(http.MethodGet, "/test/sentinel-direct", "zh-CN")
	p = testkit.AssertProblem(t, rec, http.StatusConflict)
	assert.Equal(t, "请求与当前资源状态冲突。", p.Detail)

	p = testkit.AssertProblem(t, call(http.MethodGet, "/test/sentinel-413", "zh-CN"), httperr.ContentTooLarge)
	assert.Equal(t, "请求体大小超出允许上限。", p.Detail)
	assert.NotContains(t, p.Detail, "{")

	p = testkit.AssertProblem(t, call(http.MethodGet, "/test/sentinel-415", "zh-CN"), httperr.UnsupportedMediaType)
	assert.Equal(t, "不支持的请求媒体类型。", p.Detail)
	assert.NotContains(t, p.Detail, "{")

	// 2d. oops.Public is never surfaced even for unmapped 5xx statuses, falling back to localized errors.500
	rec = call(http.MethodGet, "/test/oops-unregistered-ignores-public", "zh-CN")
	p = testkit.AssertProblem(t, rec, 599)
	assert.NotContains(t, p.Detail, "leaked public text")
	assert.Equal(t, "服务器内部错误。", p.Detail)

	// 3. oops 5xx -> localized generic message, internal error & metadata hidden
	rec = call(http.MethodGet, "/test/oops-internal", "zh-CN")
	p = testkit.AssertProblem(t, rec, http.StatusInternalServerError)
	assert.Equal(t, "服务器内部错误。", p.Detail)
	assert.NotContains(t, p.Detail, "db password leak")
	assert.Nil(t, p.Metadata)

	// 3a. Upstream timeout explicitly joined with 503 stays 503 while caller ctx is alive
	rec = call(http.MethodGet, "/test/oops-upstream-timeout", "zh-CN")
	p = testkit.AssertProblem(t, rec, httperr.ServiceUnavailable)
	assert.Equal(t, "服务暂时不可用。", p.Detail)

	// 3b. oops 5xx wrapping canceled request ctx -> classified as 499 (not 500!)
	rec = call(http.MethodGet, "/test/oops-internal-canceled-ctx", "zh-CN")
	p = testkit.AssertProblem(t, rec, httperr.ClientClosedRequest)
	assert.Equal(t, "客户端在服务端响应前关闭了连接。", p.Detail)

	// 4. *validator.FieldViolations -> 400 + ValidationInfo + localized FieldErrors
	rec = call(http.MethodGet, "/test/validation", "zh-CN")
	p = testkit.AssertProblem(t, rec, http.StatusBadRequest)
	assert.Equal(t, "请求参数校验失败。", p.Detail)
	valMeta := testkit.DecodeProblemMeta[httperr.ValidationInfo](t, p)
	assert.Equal(t, 2, valMeta.ErrorCount)
	require.Len(t, p.Errors, 2)
	assert.Equal(t, "title", p.Errors[0].Field)
	assert.Equal(t, validator.RuleRequired.Key(), p.Errors[0].Code)
	assert.Equal(t, "title 不能为空。", p.Errors[0].Message)
	assert.Nil(t, p.Errors[0].Metadata)
	assert.Equal(t, "author", p.Errors[1].Field)
	assert.Equal(t, validator.RuleMaxLength.Key(), p.Errors[1].Code)
	assert.Equal(t, "author 长度不能超过 100 个字符。", p.Errors[1].Message)
	assert.Equal(t, map[string]string{"max": "100"}, p.Errors[1].Metadata)

	// 5. *http.MaxBytesError -> 413 + RequestLimitInfo + localized detail with max_bytes
	rec = call(http.MethodGet, "/test/maxbytes", "zh-CN")
	p = testkit.AssertProblem(t, rec, http.StatusRequestEntityTooLarge)
	assert.Equal(t, "请求体大小超出允许上限（1024 字节）。", p.Detail)
	limitMeta := testkit.DecodeProblemMeta[httperr.RequestLimitInfo](t, p)
	assert.Equal(t, int64(1024), limitMeta.MaxBytes)

	// 6. context.DeadlineExceeded / context.Canceled (localized in zh-CN)
	p = testkit.AssertProblem(t, call(http.MethodGet, "/test/deadline", "zh-CN"), http.StatusGatewayTimeout)
	assert.Equal(t, "请求处理超时。", p.Detail)
	p = testkit.AssertProblem(t, call(http.MethodGet, "/test/canceled", "zh-CN"), httperr.ClientClosedRequest)
	assert.Equal(t, "客户端在服务端响应前关闭了连接。", p.Detail)

	// 7. Unknown error -> 500 with details hidden
	p = testkit.AssertProblem(t, call(http.MethodGet, "/test/unknown", ""), http.StatusInternalServerError)
	assert.NotContains(t, p.Detail, "secret database")

	// 8. Panic recovered -> 500 RFC 9457 Problem (preserving RequestID and negotiated locale)
	rec = call(http.MethodGet, "/test/panic", "zh-CN")
	p = testkit.AssertProblem(t, rec, http.StatusInternalServerError)
	assert.NotContains(t, p.Detail, "unexpected crash")
	assert.Equal(t, "服务器内部错误。", p.Detail)
	assert.NotEmpty(t, p.RequestID)
	assert.Equal(t, rec.Header().Get(httpkit.HeaderRequestID), p.RequestID)

	// 9. 404 & 405 from ServeMux converted to localized RFC 9457 Problem with ResourceInfo / MethodInfo
	p = testkit.AssertProblem(t, call(http.MethodGet, "/does-not-exist", "zh-CN"), http.StatusNotFound)
	assert.Equal(t, "/does-not-exist", p.Instance)
	assert.Equal(t, "请求的资源不存在。", p.Detail)
	nfMeta := testkit.DecodeProblemMeta[httperr.ResourceInfo](t, p)
	assert.Equal(t, httperr.ResourceInfo{ResourceType: "route", ResourceName: "/does-not-exist"}, nfMeta)

	rec = call(http.MethodPost, "/test/problem", "zh-CN")
	p = testkit.AssertProblem(t, rec, http.StatusMethodNotAllowed)
	assert.Equal(t, "该资源不支持当前 HTTP 方法。", p.Detail)
	assert.Contains(t, rec.Header().Get("Allow"), "GET")
	methodMeta := testkit.DecodeProblemMeta[httperr.MethodInfo](t, p)
	assert.Equal(t, http.MethodPost, methodMeta.Method)
	assert.Equal(t, "/test/problem", methodMeta.Path)
	assert.Contains(t, methodMeta.Allowed, http.MethodGet)
}

func TestMiddlewares_RequestIDAndLocale(t *testing.T) {
	t.Parallel()
	env := testkit.New(t)
	tr := do.MustInvoke[i18n.Translator](env.Injector)

	var gotRID, gotLocale string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRID = logging.RequestID(r.Context())
		gotLocale = i18n.Locale(r.Context(), tr.Default()).String()
		w.WriteHeader(http.StatusOK)
	})

	h := httpkit.RequestID()(server.Locale(tr)(inner))
	req := httptest.NewRequest(http.MethodGet, "/?lang=zh-CN", nil)
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set(httpkit.HeaderRequestID, "rid-999")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "rid-999", gotRID)
	assert.Equal(t, "rid-999", rec.Header().Get(httpkit.HeaderRequestID))
	assert.Equal(t, "zh-CN", gotLocale)
}
