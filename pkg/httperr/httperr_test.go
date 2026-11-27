package httperr_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/httperr"
	"github.com/liukeshao/echo-template/pkg/validator"
)

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) httperr.Problem {
	t.Helper()
	assert.Contains(t, rec.Header().Get("Content-Type"), httperr.ContentTypeProblemJSON)
	var p httperr.Problem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p), "body: %s", rec.Body.String())
	return p
}

func TestDefaultEncoder_AllClassificationPaths(t *testing.T) {
	t.Parallel()
	call := func(target string, err error) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		httperr.DefaultEncoder(rec, req, err)
		return rec
	}

	// Direct Problem
	rec := call("/problem", httperr.Problem{
		Status: http.StatusForbidden,
		Detail: "Forbidden by policy.",
	})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	p := decodeProblem(t, rec)
	assert.Equal(t, http.StatusForbidden, p.Status)
	assert.Equal(t, "/problem", p.Instance)

	// FieldViolations -> 400 + ValidationInfo + FieldErrors
	var v validator.FieldViolations
	v.Add("title", validator.RuleRequired)
	rec = call("/validation", v.Err())
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	p = decodeProblem(t, rec)
	valMeta, err := httperr.DecodeMetadata[httperr.ValidationInfo](p)
	require.NoError(t, err)
	assert.Equal(t, 1, valMeta.ErrorCount)
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "title", p.Errors[0].Field)
	assert.Equal(t, validator.RuleRequired.Key(), p.Errors[0].Code)

	// MaxBytesError -> 413 + RequestLimitInfo
	rec = call("/maxbytes", &http.MaxBytesError{Limit: 1024})
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	p = decodeProblem(t, rec)
	limitMeta, err := httperr.DecodeMetadata[httperr.RequestLimitInfo](p)
	require.NoError(t, err)
	assert.Equal(t, int64(1024), limitMeta.MaxBytes)

	// RetryInfo sets standard Retry-After header
	rec = call("/retry", httperr.Problem{
		Status:   http.StatusTooManyRequests,
		Metadata: httperr.RetryInfo{Service: "weather", RetryAfter: "30"},
	})
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "30", rec.Header().Get("Retry-After"))

	// Context errors
	assert.Equal(t, http.StatusGatewayTimeout, call("/deadline", context.DeadlineExceeded).Code)
	assert.Equal(t, httperr.ClientClosedRequest.Int(), call("/canceled", context.Canceled).Code)

	// Unknown error -> 500 without leaking raw message
	rec = call("/unknown", errors.New("secret connection string"))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	p = decodeProblem(t, rec)
	assert.NotContains(t, p.Detail, "secret")

	// Direct 5xx Problem.Detail is also sanitized by DefaultEncoder.
	rec = call("/secret-problem", httperr.Problem{Status: http.StatusInternalServerError, Detail: "raw internal trace"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	p = decodeProblem(t, rec)
	assert.NotContains(t, p.Detail, "raw internal trace")
	assert.Equal(t, "Internal Server Error", p.Detail)

	// Canceled caller request ctx wrapped with Internal -> classified as 499 (not 500).
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/canceled-caller", nil).WithContext(ctx)
	rec = httptest.NewRecorder()
	httperr.DefaultEncoder(rec, req, errors.Join(httperr.Internal, ctx.Err()))
	assert.Equal(t, httperr.ClientClosedRequest.Int(), rec.Code)
}

func TestNewEncoder_WithOptions(t *testing.T) {
	t.Parallel()
	enc := httperr.NewEncoder(
		httperr.WithRequestID(func(_ context.Context) string { return "req-42" }),
		httperr.WithMetadataExtractor(func(err error) (httperr.Metadata, bool) {
			if errors.Is(err, httperr.NotFound) {
				return httperr.ResourceInfo{ResourceType: "book", ResourceName: "01TEST"}, true
			}
			return nil, false
		}),
		httperr.WithTranslator(func(_ context.Context, key string, vars map[string]string) (string, string, bool) {
			if key == "details.404.book" {
				return "missing book " + vars["resource_name"], "en", true
			}
			if key == "validation.required" {
				return vars["field"] + " is required", "en", true
			}
			return "", "", false
		}),
	)

	rec := httptest.NewRecorder()
	enc(rec, httptest.NewRequest(http.MethodGet, "/books/01TEST", nil), httperr.NotFound)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "en", rec.Header().Get("Content-Language"))
	p := decodeProblem(t, rec)
	assert.Equal(t, "req-42", p.RequestID)
	assert.Equal(t, "missing book 01TEST", p.Detail)

	var v validator.FieldViolations
	v.Add("title", validator.RuleRequired)
	rec = httptest.NewRecorder()
	enc(rec, httptest.NewRequest(http.MethodPost, "/books", nil), v.Err())
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	p = decodeProblem(t, rec)
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "title is required", p.Errors[0].Message)
}

func TestProblem_ErrorStringStatusSentinelAndMetadata(t *testing.T) {
	t.Parallel()
	p := httperr.Problem{
		Status: http.StatusNotFound,
		Detail: "Book 01TEST was not found.",
	}
	assert.Equal(t, "http 404 Not Found: Book 01TEST was not found.", p.Error())
	assert.Equal(t, "http 404 Not Found", (httperr.Problem{Status: http.StatusNotFound}).Error())
	assert.Equal(t, 404, httperr.NotFound.Int())
	assert.Equal(t, "404", httperr.NotFound.Key())
	assert.Equal(t, "404 Not Found", httperr.NotFound.Error())
	assert.Equal(t, "404 Not Found", httperr.NotFound.String())
	assert.Len(t, httperr.AllStatuses(), 13)

	assert.True(t, httperr.Conflict.Valid())
	assert.True(t, httperr.Status(599).Valid())
	assert.False(t, httperr.Status(http.StatusOK).Valid())
	assert.False(t, httperr.Status(399).Valid())
	assert.False(t, httperr.Status(600).Valid())

	// Explicit httperr.Status takes precedence over wrapped context.DeadlineExceeded (e.g. upstream 503).
	assert.Equal(t, http.StatusServiceUnavailable, httperr.Classify(errors.Join(httperr.ServiceUnavailable, context.DeadlineExceeded)).Status)
	assert.Equal(t, http.StatusGatewayTimeout, httperr.Classify(context.DeadlineExceeded).Status)
	assert.Equal(t, http.StatusInternalServerError, httperr.Classify(httperr.Status(http.StatusOK)).Status)

	assert.Equal(t, []string{"book.book_id", "book"}, httperr.ResourceInfo{ResourceType: "book", Field: "book_id"}.I18nSubkeys())
	assert.Equal(t, []string{"book"}, httperr.ResourceInfo{ResourceType: "book"}.I18nSubkeys())
	assert.Nil(t, httperr.ResourceInfo{}.I18nSubkeys())
	assert.Equal(t, []string{"body_limit"}, httperr.RequestLimitInfo{MaxBytes: 1024}.I18nSubkeys())
	assert.Nil(t, httperr.RequestLimitInfo{}.I18nSubkeys())
	assert.Equal(t, []string{"weather"}, httperr.RetryInfo{Service: "weather"}.I18nSubkeys())
	assert.Nil(t, httperr.RetryInfo{}.I18nSubkeys())
}
