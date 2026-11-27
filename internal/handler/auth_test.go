package handler_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/internal/testutil"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/httperr"
)

func TestAuthHandler_HTTPMethods(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)

	// Register -> 201 Created
	rec := env.DoRequest(http.MethodPost, "/api/v1/auth/register",
		testutil.WithJSON(map[string]any{"username": "httpuser", "email": "http@example.com", "password": "password123"}),
	)
	require.Equal(t, http.StatusCreated, rec.Code)
	reg := testutil.DecodeJSON[api.TokenPair](t, rec)
	assert.NotEmpty(t, reg.AccessToken)

	// Malformed JSON -> localized 400 Bad Request with DecodeInfo; JSON type mismatch -> 400 Bad Request with field-level invalid_type without leaking Go struct names
	rec = env.DoRequest(http.MethodPost, "/api/v1/auth/login",
		testutil.WithBody("application/json", `{bad json`),
		testutil.WithHeader("Accept-Language", "zh-CN"),
	)
	p := testutil.AssertProblem(t, rec, http.StatusBadRequest)
	assert.Equal(t, "请求体不是合法的 JSON 格式。", p.Detail)
	decMeta := testutil.DecodeProblemMeta[httperr.DecodeInfo](t, p)
	assert.Equal(t, "invalid_json", decMeta.Reason)
	assert.Positive(t, decMeta.Offset)

	rec = env.DoRequest(http.MethodPost, "/api/v1/auth/login",
		testutil.WithBody("application/json", `{"email":123,"password":"x"}`),
	)
	p = testutil.AssertProblem(t, rec, http.StatusBadRequest)
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "email", p.Errors[0].Field)
	assert.Equal(t, "invalid_type", p.Errors[0].Code)
	assert.Equal(t, "string", p.Errors[0].Metadata["expected"])
	assert.Equal(t, "number", p.Errors[0].Metadata["actual"])
	assert.NotContains(t, p.Errors[0].Message, "LoginRequest")

	// Login
	rec = env.DoRequest(http.MethodPost, "/api/v1/auth/login",
		testutil.WithJSON(map[string]any{"email": "http@example.com", "password": "password123"}),
	)
	require.Equal(t, http.StatusOK, rec.Code)
	login := testutil.DecodeJSON[api.TokenPair](t, rec)

	// Refresh
	rec = env.DoRequest(http.MethodPost, "/api/v1/auth/refresh",
		testutil.WithJSON(map[string]any{"refresh_token": login.RefreshToken}),
	)
	require.Equal(t, http.StatusOK, rec.Code)
	refreshed := testutil.DecodeJSON[api.TokenPair](t, rec)

	// Logout -> 204
	rec = env.DoRequest(http.MethodPost, "/api/v1/auth/logout", testutil.WithBearer(refreshed.AccessToken))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
