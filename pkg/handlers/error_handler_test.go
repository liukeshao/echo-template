package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/samber/oops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/testutil"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/validator"
)

func TestErrorHandler_AllClassificationPaths(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	web := env.Echo()

	web.GET("/test/problem", func(_ *echo.Context) error {
		return types.Problem{
			Status: http.StatusForbidden,
			Code:   "POLICY_DENIED",
			Detail: "Forbidden by policy.",
		}
	})
	web.GET("/test/oops-localized", func(_ *echo.Context) error {
		return oops.
			Code(codes.BookNotFound.String()).
			With(codes.CtxKeyParams, map[string]string{"book_id": "01TEST"}).
			Public("Book not found").
			Errorf("book not found")
	})
	web.GET("/test/oops-internal", func(_ *echo.Context) error {
		return oops.
			Code(codes.BookInternal.String()).
			With(codes.CtxKeyParams, map[string]string{"sql": "SELECT secret"}).
			Wrapf(errors.New("db password leak"), "querying book")
	})
	web.GET("/test/validation", func(_ *echo.Context) error {
		var v validator.FieldViolations
		v.Add("title", validator.KeyRequired, map[string]string{"field": "title"})
		return v.Err()
	})
	web.GET("/test/deadline", func(_ *echo.Context) error {
		return context.DeadlineExceeded
	})
	web.GET("/test/canceled", func(_ *echo.Context) error {
		return context.Canceled
	})
	web.GET("/test/echo-http", func(_ *echo.Context) error {
		return echo.NewHTTPError(http.StatusForbidden, "custom forbidden")
	})
	web.GET("/test/unknown", func(_ *echo.Context) error {
		return errors.New("secret database connection string")
	})
	web.GET("/test/committed", func(c *echo.Context) error {
		_ = c.String(http.StatusAccepted, "already sent")
		return errors.New("late error")
	})

	// 1. Direct types.Problem -> defaults filled (type, title, instance, request_id)
	rec := env.DoRequest(http.MethodGet, "/test/problem")
	p := testutil.AssertProblem(t, rec, http.StatusForbidden, "POLICY_DENIED")
	assert.Equal(t, "about:blank", p.Type)
	assert.Equal(t, "Forbidden", p.Title)
	assert.Equal(t, "Forbidden by policy.", p.Detail)
	assert.Equal(t, "/test/problem", p.Instance)
	assert.Equal(t, rec.Header().Get(echo.HeaderXRequestID), p.RequestID)

	// 2. oops with CtxKeyParams -> localized Chinese detail + exposed params
	rec = env.DoRequest(http.MethodGet, "/test/oops-localized", testutil.WithHeader("Accept-Language", "zh-CN"))
	p = testutil.AssertProblem(t, rec, http.StatusNotFound, "BOOK_NOT_FOUND")
	assert.Equal(t, "Not Found", p.Title)
	assert.Equal(t, "未找到 ID 为 01TEST 的书籍。", p.Detail)
	assert.Equal(t, map[string]string{"book_id": "01TEST"}, p.Params)

	// 3. oops 5xx -> localized generic message, internal error & params hidden
	rec = env.DoRequest(http.MethodGet, "/test/oops-internal", testutil.WithHeader("Accept-Language", "zh-CN"))
	p = testutil.AssertProblem(t, rec, http.StatusInternalServerError, "BOOK_INTERNAL_ERROR")
	assert.Equal(t, "处理书籍请求时发生内部错误。", p.Detail)
	assert.NotContains(t, p.Detail, "db password leak")
	assert.Empty(t, p.Params)

	// 4. *validator.FieldViolations -> localized 400 VALIDATION_INVALID + field errors
	rec = env.DoRequest(http.MethodGet, "/test/validation", testutil.WithHeader("Accept-Language", "zh-CN"))
	p = testutil.AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_INVALID")
	assert.Equal(t, "请求参数校验失败。", p.Detail)
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "title", p.Errors[0].Field)
	assert.Equal(t, "title 不能为空。", p.Errors[0].Message)

	// 5. context.DeadlineExceeded / context.Canceled
	rec = env.DoRequest(http.MethodGet, "/test/deadline")
	testutil.AssertProblem(t, rec, http.StatusGatewayTimeout, "GATEWAY_TIMEOUT")

	rec = env.DoRequest(http.MethodGet, "/test/canceled")
	testutil.AssertProblem(t, rec, 499, "CLIENT_CLOSED_REQUEST")

	// 6. *echo.HTTPError
	rec = env.DoRequest(http.MethodGet, "/test/echo-http")
	p = testutil.AssertProblem(t, rec, http.StatusForbidden, "FORBIDDEN")
	assert.Equal(t, "custom forbidden", p.Detail)

	// 7. Unknown error -> 500 INTERNAL_ERROR with details hidden
	rec = env.DoRequest(http.MethodGet, "/test/unknown")
	p = testutil.AssertProblem(t, rec, http.StatusInternalServerError, "INTERNAL_ERROR")
	assert.NotContains(t, p.Detail, "secret database")

	// 8. Committed response is not overwritten
	rec = httptest.NewRecorder()
	web.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/test/committed", nil))
	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "already sent", rec.Body.String())
}
