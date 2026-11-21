package testutil

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/samber/oops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/handlers"
)

// AssertOopsCode 断言 err 包含指定的业务错误码（oops.OopsError）。
func AssertOopsCode(t testing.TB, err error, want codes.Code) {
	t.Helper()
	require.Error(t, err)
	oe, ok := errors.AsType[oops.OopsError](err)
	require.True(t, ok, "expected oops error, got %T: %v", err, err)
	assert.Equal(t, want.String(), oe.Code())
}

// DecodeJSON 将 HTTP 响应体反序列化为目标类型 T。
func DecodeJSON[T any](t testing.TB, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), "body: %s", rec.Body.String())
	return out
}

// AssertAPIError 断言响应符合 AIP-193 错误信封格式并返回解析后的 ErrorResponse 供进一步断言。
func AssertAPIError(t testing.TB, rec *httptest.ResponseRecorder, wantHTTP int, wantStatus string) handlers.ErrorResponse {
	t.Helper()
	assert.Equal(t, wantHTTP, rec.Code, "body: %s", rec.Body.String())
	resp := DecodeJSON[handlers.ErrorResponse](t, rec)
	assert.Equal(t, wantHTTP, resp.Error.Code)
	assert.Equal(t, wantStatus, resp.Error.Status)
	return resp
}
