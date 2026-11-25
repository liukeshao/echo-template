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
	"github.com/liukeshao/echo-template/pkg/rest"
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

// AssertProblem 断言响应符合 RFC 9457 Problem Details 格式（application/problem+json），
// 并返回解析后的 rest.Problem 供调用方进一步断言。
func AssertProblem(t testing.TB, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) rest.Problem {
	t.Helper()
	assert.Equal(t, wantStatus, rec.Code, "body: %s", rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), rest.ContentTypeProblemJSON)
	p := DecodeJSON[rest.Problem](t, rec)
	assert.Equal(t, wantStatus, p.Status)
	assert.Equal(t, wantCode, p.Code)
	assert.NotEmpty(t, p.Title)
	assert.NotEmpty(t, p.Detail)
	return p
}
