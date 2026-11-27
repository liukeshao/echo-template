package testutil

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/samber/oops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/httperr"
)

// AssertOopsStatus 断言 err 同时满足：
//   - 为 oops.OopsError（保留域上下文与调用栈）；
//   - errors.Is(err, want) 且错误链中提取出的主 httperr.Status 等于 want；
//
// 并返回提取出的 OopsError 供调用方进一步校验 Metadata / Context。
func AssertOopsStatus(t testing.TB, err error, want httperr.Status) oops.OopsError {
	t.Helper()
	require.Error(t, err)
	oe, ok := oops.AsOops(err)
	require.True(t, ok, "expected oops error, got %T: %v", err, err)
	assert.ErrorIs(t, err, want, "expected error chain to match sentinel %s", want)
	got, ok := errors.AsType[httperr.Status](err)
	require.True(t, ok, "expected httperr.Status sentinel in error chain, got %v", err)
	assert.Equal(t, want, got)
	return oe
}

// DecodeJSON 将 HTTP 响应体反序列化为目标类型 T。
func DecodeJSON[T any](t testing.TB, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), "body: %s", rec.Body.String())
	return out
}

// AssertProblem 断言响应符合 RFC 9457 Problem Details 格式（application/problem+json），
// 并返回解析后的 httperr.Problem 供调用方进一步断言。
// want 接受 httperr.Status 或任意 net/http 无类型状态码常量（如 http.StatusNotFound）。
func AssertProblem(t testing.TB, rec *httptest.ResponseRecorder, want httperr.Status) httperr.Problem {
	t.Helper()
	assert.Equal(t, want.Int(), rec.Code, "body: %s", rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), httperr.ContentTypeProblemJSON)
	p := DecodeJSON[httperr.Problem](t, rec)
	assert.Equal(t, want.Int(), p.Status)
	assert.NotEmpty(t, p.Title)
	assert.NotEmpty(t, p.Detail)
	return p
}

// DecodeProblemMeta 将 Problem.Metadata 解码为强类型 AIP-193 Metadata 结构体 T。
func DecodeProblemMeta[T any](t testing.TB, p httperr.Problem) T {
	t.Helper()
	meta, err := httperr.DecodeMetadata[T](p)
	require.NoError(t, err, "problem status=%d metadata=%#v", p.Status, p.Metadata)
	return meta
}
