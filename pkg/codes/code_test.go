package codes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/liukeshao/echo-template/pkg/codes"
	rpccodes "github.com/liukeshao/echo-template/transport/rpc/codes"
)

func TestLookup(t *testing.T) {
	t.Parallel()
	assert.Equal(t, rpccodes.NotFound, codes.Lookup(codes.BookNotFound))
	assert.Equal(t, rpccodes.AlreadyExists, codes.Lookup(codes.BookAlreadyExists))
	assert.Equal(t, rpccodes.Unauthenticated, codes.Lookup(codes.AuthTokenInvalid))
	assert.Equal(t, rpccodes.ResourceExhausted, codes.Lookup(codes.WeatherRateLimited))
	assert.Equal(t, rpccodes.Internal, codes.Lookup(codes.Code("UNREGISTERED_CODE")))
}

func TestRegisteredCodes(t *testing.T) {
	t.Parallel()
	all := codes.RegisteredCodes()
	assert.Contains(t, all, codes.BookNotFound)
	assert.Contains(t, all, codes.AuthInvalidCredentials)
	assert.Contains(t, all, codes.ValidationInvalid)
	assert.Contains(t, all, codes.PatchInvalid)
}

func TestRegister_DuplicatePanics(t *testing.T) {
	assert.Panics(t, func() {
		codes.Register(codes.BookNotFound, rpccodes.NotFound)
	})
}
