package testutil

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/samber/oops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/rest"
)

// AssertOopsCode 断言 err 同时满足：
//   - errors.Is(err, want) 哨兵匹配；
//   - 为 oops.OopsError 且 Code() 等于 want；
//
// 并返回提取出的 OopsError 供调用方进一步校验 Metadata / Context。
func AssertOopsCode(t testing.TB, err error, want rest.Code) oops.OopsError {
	t.Helper()
	require.Error(t, err)
	assert.ErrorIs(t, err, want, "expected error chain to match sentinel %s", want)
	oe, ok := oops.AsOops(err)
	require.True(t, ok, "expected oops error, got %T: %v", err, err)
	got, ok := rest.CodeFromAny(oe.Code())
	require.True(t, ok, "expected oops error code, got %v", oe.Code())
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
// 并返回解析后的 rest.Problem 供调用方进一步断言。
func AssertProblem(t testing.TB, rec *httptest.ResponseRecorder, wantStatus int, wantCode rest.Code) rest.Problem {
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

// DecodeProblemMeta 将 Problem.Metadata 解码为强类型 AIP-193 Metadata 结构体 T。
func DecodeProblemMeta[T any](t testing.TB, p rest.Problem) T {
	t.Helper()
	meta, err := rest.DecodeMetadata[T](p)
	require.NoError(t, err, "problem code=%s metadata=%#v", p.Code, p.Metadata)
	return meta
}
