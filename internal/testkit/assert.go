package testkit

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/samber/oops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/go-rest-template/pkg/httperr"
)

// AssertOopsStatus 断言 err 为 oops.OopsError 且主哨兵状态码等于 want。
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

func DecodeJSON[T any](t testing.TB, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), "body: %s", rec.Body.String())
	return out
}

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

func DecodeProblemMeta[T any](t testing.TB, p httperr.Problem) T {
	t.Helper()
	meta, err := httperr.DecodeMetadata[T](p)
	require.NoError(t, err, "problem status=%d metadata=%#v", p.Status, p.Metadata)
	return meta
}
