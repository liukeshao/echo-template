package status_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpstatus "github.com/liukeshao/echo-template/transport/http/status"
	"github.com/liukeshao/echo-template/transport/rpc/codes"
	"github.com/liukeshao/echo-template/transport/rpc/errdetails"
	rpcstatus "github.com/liukeshao/echo-template/transport/rpc/status"
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

func TestFromRPCStatus_AndErrorDetailsRoundTrip(t *testing.T) {
	t.Parallel()

	s := rpcstatus.New(codes.NotFound, "Book not found").WithDetails(
		errdetails.ErrorInfo{
			Type:     errdetails.TypeErrorInfo,
			Reason:   "BOOK_NOT_FOUND",
			Domain:   "api.example.com",
			Metadata: map[string]string{"book_id": "01TEST"},
		},
		errdetails.LocalizedMessage{
			Type:    errdetails.TypeLocalizedMessage,
			Locale:  "zh-CN",
			Message: "未找到书籍",
		},
		errdetails.BadRequest{
			Type: errdetails.TypeBadRequest,
			FieldViolations: []errdetails.FieldViolation{
				{Field: "title", Description: "required"},
			},
		},
		errdetails.ResourceInfo{
			Type:         errdetails.TypeResourceInfo,
			ResourceType: "Book",
			ResourceName: "01TEST",
		},
		errdetails.RequestInfo{
			Type:      errdetails.TypeRequestInfo,
			RequestID: "req-123",
		},
		map[string]any{
			"@type": "type.googleapis.com/example.CustomDetail",
			"extra": "kept",
		},
	)

	httpCode, resp := httpstatus.FromRPCStatus(s)
	assert.Equal(t, http.StatusNotFound, httpCode)
	assert.Equal(t, "http 404 (NOT_FOUND): Book not found", resp.Error.Error())

	raw, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded httpstatus.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.Equal(t, http.StatusNotFound, decoded.Error.Code)
	assert.Equal(t, "NOT_FOUND", decoded.Error.Status)
	assert.Equal(t, "Book not found", decoded.Error.Message)

	ei, ok := decoded.Error.ErrorInfo()
	require.True(t, ok)
	assert.Equal(t, "BOOK_NOT_FOUND", ei.Reason)
	assert.Equal(t, "01TEST", ei.Metadata["book_id"])

	lm, ok := decoded.Error.LocalizedMessage()
	require.True(t, ok)
	assert.Equal(t, "zh-CN", lm.Locale)
	assert.Equal(t, "未找到书籍", lm.Message)

	br, ok := decoded.Error.BadRequest()
	require.True(t, ok)
	require.Len(t, br.FieldViolations, 1)
	assert.Equal(t, "title", br.FieldViolations[0].Field)

	ri, ok := decoded.Error.ResourceInfo()
	require.True(t, ok)
	assert.Equal(t, "Book", ri.ResourceType)
	assert.Equal(t, "01TEST", ri.ResourceName)

	reqInfo, ok := decoded.Error.RequestInfo()
	require.True(t, ok)
	assert.Equal(t, "req-123", reqInfo.RequestID)

	// Unknown @type is preserved as json.RawMessage.
	custom, ok := httpstatus.FindDetail[json.RawMessage](decoded.Error.Details)
	require.True(t, ok)
	assert.Contains(t, string(custom), "example.CustomDetail")

	// Empty details unmarshal to nil slice.
	var empty httpstatus.ErrorResponse
	require.NoError(t, json.Unmarshal([]byte(`{"error":{"code":500,"status":"INTERNAL","message":"boom","details":[]}}`), &empty))
	assert.Nil(t, empty.Error.Details)
	_, ok = empty.Error.ErrorInfo()
	assert.False(t, ok)
}
