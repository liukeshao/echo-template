package codes_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/liukeshao/echo-template/internal/codes"
	"github.com/liukeshao/echo-template/pkg/rest"
)

func TestHTTPStatus(t *testing.T) {
	t.Parallel()
	assert.Equal(t, http.StatusNotFound, codes.HTTPStatus(codes.BookNotFound))
	assert.Equal(t, http.StatusConflict, codes.HTTPStatus(codes.BookAlreadyExists))
	assert.Equal(t, http.StatusUnauthorized, codes.HTTPStatus(codes.AuthTokenInvalid))
	assert.Equal(t, http.StatusUnsupportedMediaType, codes.HTTPStatus(codes.PatchUnsupportedMediaType))
	assert.Equal(t, http.StatusTooManyRequests, codes.HTTPStatus(codes.WeatherRateLimited))
	assert.Equal(t, http.StatusBadRequest, codes.HTTPStatus(codes.BadRequest))
	assert.Equal(t, rest.StatusClientClosedRequest, codes.HTTPStatus(codes.ClientClosedRequest))
	assert.Equal(t, http.StatusInternalServerError, codes.HTTPStatus(codes.Code("UNREGISTERED_CODE")))
}

func TestFromAny(t *testing.T) {
	t.Parallel()
	c, ok := codes.FromAny(codes.BookNotFound)
	assert.True(t, ok)
	assert.Equal(t, codes.BookNotFound, c)

	c, ok = codes.FromAny("BOOK_NOT_FOUND")
	assert.True(t, ok)
	assert.Equal(t, codes.BookNotFound, c)

	_, ok = codes.FromAny("")
	assert.False(t, ok)
	_, ok = codes.FromAny(codes.Code(""))
	assert.False(t, ok)
	_, ok = codes.FromAny(123)
	assert.False(t, ok)
}

func TestRegisteredCodes(t *testing.T) {
	t.Parallel()
	all := codes.RegisteredCodes()
	assert.Contains(t, all, codes.BookNotFound)
	assert.Contains(t, all, codes.AuthInvalidCredentials)
	assert.Contains(t, all, codes.ValidationInvalid)
	assert.Contains(t, all, codes.PatchInvalid)
	assert.Contains(t, all, codes.BadRequest)
	assert.Contains(t, all, codes.NotFound)
	assert.Contains(t, all, codes.MethodNotAllowed)
	assert.Contains(t, all, codes.RequestEntityTooLarge)
	assert.Contains(t, all, codes.GatewayTimeout)
	assert.Contains(t, all, codes.ClientClosedRequest)
	assert.Contains(t, all, codes.InternalError)
}

func TestRegister_PanicsOnDuplicateOrInvalidStatus(t *testing.T) {
	assert.Panics(t, func() {
		codes.Register(codes.BookNotFound, http.StatusNotFound)
	})
	assert.Panics(t, func() {
		codes.Register(codes.Code("TEST_INVALID_STATUS"), http.StatusOK)
	})
}
