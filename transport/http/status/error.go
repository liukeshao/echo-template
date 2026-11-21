package status

import (
	"encoding/json"
	"fmt"

	"github.com/liukeshao/echo-template/transport/rpc/errdetails"
	rpcstatus "github.com/liukeshao/echo-template/transport/rpc/status"
)

// ErrorResponse is the top-level AIP-193 HTTP/JSON error envelope.
//
// All error responses from this API share this structure:
//
//	{
//	  "error": {
//	    "code": 429,
//	    "message": "...",
//	    "status": "RESOURCE_EXHAUSTED",
//	    "details": [
//	      { "@type": "type.googleapis.com/google.rpc.ErrorInfo", ... },
//	      { "@type": "type.googleapis.com/google.rpc.LocalizedMessage", ... },
//	      { "@type": "type.googleapis.com/google.rpc.RequestInfo", ... }
//	    ]
//	  }
//	}
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the AIP-193 Status payload embedded under the "error" key.
type ErrorBody struct {
	// HTTP status code, e.g. 404. Not the google.rpc.Code numeric value.
	Code int `json:"code" example:"404"`
	// Developer-facing English description of the error.
	Message string `json:"message" example:"requested entity was not found"`
	// Canonical google.rpc.Code name in UPPER_SNAKE_CASE, e.g. "NOT_FOUND".
	Status string `json:"status" example:"NOT_FOUND"`
	// AIP-193 typed detail payloads. The "@type" field discriminates the entry:
	//   - type.googleapis.com/google.rpc.ErrorInfo        — always present (see errdetails.ErrorInfo)
	//   - type.googleapis.com/google.rpc.LocalizedMessage — user-facing localized message (see errdetails.LocalizedMessage)
	//   - type.googleapis.com/google.rpc.BadRequest       — field-level validation errors (see errdetails.BadRequest)
	//   - type.googleapis.com/google.rpc.ResourceInfo     — resource context for NOT_FOUND / ALREADY_EXISTS (see errdetails.ResourceInfo)
	//   - type.googleapis.com/google.rpc.RequestInfo      — request identifier for log correlation (see errdetails.RequestInfo)
	Details ErrorDetails `json:"details,omitempty"`
}

// Error implements the error interface so HTTP clients can return an unmarshaled
// resp.Error directly as an error.
func (b ErrorBody) Error() string {
	return fmt.Sprintf("http %d (%s): %s", b.Code, b.Status, b.Message)
}

// ErrorInfo returns the first errdetails.ErrorInfo in Details, if present.
func (b ErrorBody) ErrorInfo() (errdetails.ErrorInfo, bool) {
	return FindDetail[errdetails.ErrorInfo](b.Details)
}

// LocalizedMessage returns the first errdetails.LocalizedMessage in Details, if present.
func (b ErrorBody) LocalizedMessage() (errdetails.LocalizedMessage, bool) {
	return FindDetail[errdetails.LocalizedMessage](b.Details)
}

// BadRequest returns the first errdetails.BadRequest in Details, if present.
func (b ErrorBody) BadRequest() (errdetails.BadRequest, bool) {
	return FindDetail[errdetails.BadRequest](b.Details)
}

// ResourceInfo returns the first errdetails.ResourceInfo in Details, if present.
func (b ErrorBody) ResourceInfo() (errdetails.ResourceInfo, bool) {
	return FindDetail[errdetails.ResourceInfo](b.Details)
}

// RequestInfo returns the first errdetails.RequestInfo in Details, if present.
func (b ErrorBody) RequestInfo() (errdetails.RequestInfo, bool) {
	return FindDetail[errdetails.RequestInfo](b.Details)
}

// ErrorDetails holds AIP-193 detail payloads.
// On JSON unmarshaling, known "@type" entries are decoded into their concrete
// errdetails structs; unknown entries are preserved as json.RawMessage.
type ErrorDetails []any

// UnmarshalJSON decodes a JSON array of AIP-193 detail objects into typed errdetails values.
func (d *ErrorDetails) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) == 0 {
		*d = nil
		return nil
	}

	out := make(ErrorDetails, 0, len(raw))
	for _, item := range raw {
		var envelope struct {
			Type string `json:"@type"`
		}
		if err := json.Unmarshal(item, &envelope); err != nil {
			out = append(out, item)
			continue
		}

		decoded, err := unmarshalDetail(envelope.Type, item)
		if err != nil {
			return err
		}
		out = append(out, decoded)
	}
	*d = out
	return nil
}

func unmarshalDetail(typeURI string, raw json.RawMessage) (any, error) {
	switch typeURI {
	case errdetails.TypeErrorInfo:
		var v errdetails.ErrorInfo
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return v, nil
	case errdetails.TypeLocalizedMessage:
		var v errdetails.LocalizedMessage
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return v, nil
	case errdetails.TypeBadRequest:
		var v errdetails.BadRequest
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return v, nil
	case errdetails.TypeResourceInfo:
		var v errdetails.ResourceInfo
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return v, nil
	case errdetails.TypeRequestInfo:
		var v errdetails.RequestInfo
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return v, nil
	default:
		return raw, nil
	}
}

// FindDetail returns the first element of type T in details.
func FindDetail[T any](details ErrorDetails) (T, bool) {
	for _, item := range details {
		if v, ok := item.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// FromRPCStatus converts an immutable *rpcstatus.Status into its HTTP status code
// and AIP-193 ErrorResponse envelope.
func FromRPCStatus(s *rpcstatus.Status) (int, ErrorResponse) {
	httpCode := FromRPCCode(s.Code())
	return httpCode, ErrorResponse{
		Error: ErrorBody{
			Code:    httpCode,
			Message: s.Message(),
			Status:  s.Code().Name(),
			Details: ErrorDetails(s.Details()),
		},
	}
}
