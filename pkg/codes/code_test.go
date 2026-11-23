package codes_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/liukeshao/echo-template/pkg/codes"
)

func TestHTTPStatus(t *testing.T) {
	t.Parallel()
	assert.Equal(t, http.StatusNotFound, codes.HTTPStatus(codes.BookNotFound))
	assert.Equal(t, http.StatusConflict, codes.HTTPStatus(codes.BookAlreadyExists))
	assert.Equal(t, http.StatusUnauthorized, codes.HTTPStatus(codes.AuthTokenInvalid))
	assert.Equal(t, http.StatusUnsupportedMediaType, codes.HTTPStatus(codes.PatchUnsupportedMediaType))
	assert.Equal(t, http.StatusTooManyRequests, codes.HTTPStatus(codes.WeatherRateLimited))
	assert.Equal(t, http.StatusInternalServerError, codes.HTTPStatus(codes.Code("UNREGISTERED_CODE")))
}

func TestRegisteredCodes(t *testing.T) {
	t.Parallel()
	all := codes.RegisteredCodes()
	assert.Contains(t, all, codes.BookNotFound)
	assert.Contains(t, all, codes.AuthInvalidCredentials)
	assert.Contains(t, all, codes.ValidationInvalid)
	assert.Contains(t, all, codes.PatchInvalid)
}

func TestRegister_PanicsOnDuplicateOrInvalidStatus(t *testing.T) {
	assert.Panics(t, func() {
		codes.Register(codes.BookNotFound, http.StatusNotFound)
	})
	assert.Panics(t, func() {
		codes.Register(codes.Code("TEST_INVALID_STATUS"), http.StatusOK)
	})
}
