package status_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	httpstatus "github.com/liukeshao/echo-template/transport/http/status"
	"github.com/liukeshao/echo-template/transport/rpc/codes"
)

func TestFromRPCCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		rpc  codes.Code
		want int
	}{
		{codes.OK, http.StatusOK},
		{codes.Canceled, 499},
		{codes.Unknown, http.StatusInternalServerError},
		{codes.InvalidArgument, http.StatusBadRequest},
		{codes.DeadlineExceeded, http.StatusGatewayTimeout},
		{codes.NotFound, http.StatusNotFound},
		{codes.AlreadyExists, http.StatusConflict},
		{codes.PermissionDenied, http.StatusForbidden},
		{codes.ResourceExhausted, http.StatusTooManyRequests},
		{codes.FailedPrecondition, http.StatusBadRequest},
		{codes.Aborted, http.StatusConflict},
		{codes.OutOfRange, http.StatusBadRequest},
		{codes.Unimplemented, http.StatusNotImplemented},
		{codes.Internal, http.StatusInternalServerError},
		{codes.Unavailable, http.StatusServiceUnavailable},
		{codes.DataLoss, http.StatusInternalServerError},
		{codes.Unauthenticated, http.StatusUnauthorized},
		{codes.Code(999), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, httpstatus.FromRPCCode(tc.rpc))
	}
}

func TestToRPCCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		httpCode int
		want     codes.Code
	}{
		{http.StatusOK, codes.OK},
		{http.StatusBadRequest, codes.InvalidArgument},
		{http.StatusUnauthorized, codes.Unauthenticated},
		{http.StatusForbidden, codes.PermissionDenied},
		{http.StatusNotFound, codes.NotFound},
		{http.StatusMethodNotAllowed, codes.Unimplemented},
		{http.StatusConflict, codes.AlreadyExists},
		{http.StatusRequestEntityTooLarge, codes.ResourceExhausted},
		{http.StatusTooManyRequests, codes.ResourceExhausted},
		{http.StatusNotImplemented, codes.Unimplemented},
		{http.StatusServiceUnavailable, codes.Unavailable},
		{http.StatusGatewayTimeout, codes.DeadlineExceeded},
		{http.StatusInternalServerError, codes.Internal},
		{http.StatusTeapot, codes.Unknown},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, httpstatus.ToRPCCode(tc.httpCode))
	}
}
