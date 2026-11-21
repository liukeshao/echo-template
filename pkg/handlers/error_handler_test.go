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
	"github.com/liukeshao/echo-template/pkg/validator"
	rpccodes "github.com/liukeshao/echo-template/transport/rpc/codes"
	"github.com/liukeshao/echo-template/transport/rpc/errdetails"
	rpcstatus "github.com/liukeshao/echo-template/transport/rpc/status"
)

func TestErrorHandler_AllClassificationPaths(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	web := env.Echo()

	web.GET("/test/rpc", func(_ *echo.Context) error {
		return rpcstatus.New(rpccodes.PermissionDenied, "forbidden by policy")
	})
	web.GET("/test/oops-resource", func(_ *echo.Context) error {
		return oops.
			Code(codes.BookNotFound.String()).
			With(codes.CtxKeyResourceInfo, errdetails.ResourceInfo{
				ResourceType: "Book",
				ResourceName: "01TEST",
			}).
			With(codes.CtxKeyMetadata, map[string]string{"book_id": "01TEST"}).
			Public("Book not found").
			Errorf("not found")
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

	// 1. *rpcstatus.Status — includes fallback ErrorInfo and RequestInfo
	rec := env.DoRequest(http.MethodGet, "/test/rpc")
	resp := testutil.AssertAPIError(t, rec, http.StatusForbidden, "PERMISSION_DENIED")
	ei, ok := resp.Error.ErrorInfo()
	require.True(t, ok)
	assert.Equal(t, "PERMISSION_DENIED", ei.Reason)
	reqInfo, ok := resp.Error.RequestInfo()
	require.True(t, ok)
	assert.Equal(t, rec.Header().Get(echo.HeaderXRequestID), reqInfo.RequestID)

	// 2. oops with ResourceInfo omitting Type -> Type defaulted + Chinese LocalizedMessage
	rec = env.DoRequest(http.MethodGet, "/test/oops-resource", testutil.WithHeader("Accept-Language", "zh-CN"))
	resp = testutil.AssertAPIError(t, rec, http.StatusNotFound, "NOT_FOUND")
	ri, ok := resp.Error.ResourceInfo()
	require.True(t, ok)
	assert.Equal(t, errdetails.TypeResourceInfo, ri.Type)
	assert.Equal(t, "Book", ri.ResourceType)
	assert.Equal(t, "01TEST", ri.ResourceName)
	lm, ok := resp.Error.LocalizedMessage()
	require.True(t, ok)
	assert.Equal(t, "未找到 ID 为 01TEST 的书籍。", lm.Message)

	// 3. *validator.FieldViolations -> localized BadRequest + LocalizedMessage
	rec = env.DoRequest(http.MethodGet, "/test/validation", testutil.WithHeader("Accept-Language", "zh-CN"))
	resp = testutil.AssertAPIError(t, rec, http.StatusBadRequest, "INVALID_ARGUMENT")
	br, ok := resp.Error.BadRequest()
	require.True(t, ok)
	require.Len(t, br.FieldViolations, 1)
	assert.Equal(t, "title", br.FieldViolations[0].Field)
	assert.Equal(t, "title 不能为空。", br.FieldViolations[0].Description)

	// 4. context.DeadlineExceeded / context.Canceled
	rec = env.DoRequest(http.MethodGet, "/test/deadline")
	testutil.AssertAPIError(t, rec, http.StatusGatewayTimeout, "DEADLINE_EXCEEDED")

	rec = env.DoRequest(http.MethodGet, "/test/canceled")
	testutil.AssertAPIError(t, rec, 499, "CANCELLED")

	// 5. *echo.HTTPError
	rec = env.DoRequest(http.MethodGet, "/test/echo-http")
	resp = testutil.AssertAPIError(t, rec, http.StatusForbidden, "PERMISSION_DENIED")
	assert.Equal(t, "custom forbidden", resp.Error.Message)

	// 6. Unknown error -> 500 Internal with details hidden
	rec = env.DoRequest(http.MethodGet, "/test/unknown")
	resp = testutil.AssertAPIError(t, rec, http.StatusInternalServerError, "INTERNAL")
	assert.NotContains(t, resp.Error.Message, "secret database")

	// 7. Committed response is not overwritten
	rec = httptest.NewRecorder()
	web.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/test/committed", nil))
	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "already sent", rec.Body.String())
}
