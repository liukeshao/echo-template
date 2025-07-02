package errors

import "fmt"

// ErrorDetail provides details about a specific error.
type ErrorDetail struct {
	// Message is a human-readable explanation of the error.
	Message string `json:"message,omitempty" doc:"Error message text"`

	// Location is a path-like string indicating where the error occurred.
	// It typically begins with `path`, `query`, `header`, or `body`. Example:
	// `body.items[3].tags` or `path.thing-id`.
	Location string `json:"location,omitempty" doc:"Where the error occurred, e.g. 'body.items[3].tags' or 'path.thing-id'"`

	// Value is the value at the given location, echoed back to the client
	// to help with debugging. This can be useful for e.g. validating that
	// the client didn't send extra whitespace or help when the client
	// did not log an outgoing request.
	Value any `json:"value,omitempty" doc:"The value at the given location"`
}

// Error returns the error message / satisfies the `error` interface. If a
// location and value are set, they will be included in the error message,
// otherwise just the message is returned.
func (e *ErrorDetail) Error() string {
	if e.Location == "" && e.Value == nil {
		return e.Message
	}
	return fmt.Sprintf("%s (%s: %v)", e.Message, e.Location, e.Value)
}
