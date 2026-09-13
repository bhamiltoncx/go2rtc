package nest

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func fakeResponse(code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Status:     http.StatusText(code),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestStatusErrorCarriesGoogleReason(t *testing.T) {
	err := newStatusError(fakeResponse(400, `{"error":{"code":400,"message":"The camera is not available for streaming.","status":"FAILED_PRECONDITION"}}`))

	var se *StatusError
	require.ErrorAs(t, err, &se)
	require.Equal(t, 400, se.Code)
	require.Contains(t, err.Error(), "nest: wrong status: Bad Request")
	require.Contains(t, err.Error(), "The camera is not available for streaming.")
}

// A camera that is switched off, missing, or forbidden will not come back
// within a retry window; only transport failures and server errors deserve one.
func TestRetryable(t *testing.T) {
	require.False(t, retryable(newStatusError(fakeResponse(400, ""))))
	require.False(t, retryable(newStatusError(fakeResponse(403, ""))))
	require.False(t, retryable(newStatusError(fakeResponse(404, ""))))
	require.True(t, retryable(newStatusError(fakeResponse(500, ""))))
	require.True(t, retryable(newStatusError(fakeResponse(503, ""))))
	require.True(t, retryable(errors.New("dial tcp: i/o timeout")))
}
