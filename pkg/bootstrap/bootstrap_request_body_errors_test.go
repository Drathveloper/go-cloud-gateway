package bootstrap_test

import (
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// oneRequestRateLimit lets a single request through and rejects the next ones for
	// a second.
	oneRequestRateLimit = `,"filters":[{"name":"RateLimit","args":{"type":"in-memory","key":"ip","rate":1,"burst":1}}]`
	// firstFailureBreaker opens at the first failed request and stays open.
	firstFailureBreaker = `,"circuit-breaker":{"enabled":true,"interval":"1m","min-requests-threshold":1,` +
		`"failure-rate-threshold":1,"wait-duration-in-open-state":"1m","num-allowed-half-open-calls":1}`
)

// newUnreachableBackend returns the URL of a backend that refuses connections.
func newUnreachableBackend(t *testing.T) string {
	t.Helper()
	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, listener.Close())
	return "http://" + listener.Addr().String()
}

// rejectAtOnce answers before it has read the request body, like a backend that turns
// down an upload by its headers.
func rejectAtOnce(writer http.ResponseWriter, _ *http.Request) {
	controller := http.NewResponseController(writer)
	if err := controller.EnableFullDuplex(); err != nil {
		return
	}
	writer.Header().Set("Content-Length", "0")
	writer.WriteHeader(http.StatusRequestEntityTooLarge)
	_ = controller.Flush()
}

// warmUp sends a request without a body, which the tests use to spend the rate limit or
// to open the circuit breaker before the request they are about.
func warmUp(t *testing.T, client *http.Client, gatewayURL string) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, gatewayURL+"/warm-up", nil)
	require.NoError(t, err)
	res, err := client.Do(request)
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
}

// net/http reads what is left of a request body before it writes the response. A
// response that does not need the rest of the body, because the gateway rejected the
// request or the backend answered early, was only sent when the upload ended or the
// server read timeout expired. It now waits for a moment at most.
func TestInitialize_ResponsesDoNotWaitForARequestBodyThatIsStillSent(t *testing.T) {
	// The upload takes 600ms: a response held back until its end is told apart easily.
	upload := slowUpload{size: 30, interval: 20 * time.Millisecond}
	const uploadTime = 600 * time.Millisecond
	tests := []struct {
		backend        func(t *testing.T) string
		name           string
		routeExtra     string
		expectedStatus int
		warmUp         bool
	}{
		{
			name:           "backend that cannot be reached",
			backend:        newUnreachableBackend,
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "rate limit exceeded",
			backend:        func(t *testing.T) string { t.Helper(); return newBodyTestBackend(t, answerAfterTheBody) },
			routeExtra:     oneRequestRateLimit,
			warmUp:         true,
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "circuit breaker open",
			backend:        newUnreachableBackend,
			routeExtra:     firstFailureBreaker,
			warmUp:         true,
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "backend that answers before the request body",
			backend:        func(t *testing.T) string { t.Helper(); return newBodyTestBackend(t, rejectAtOnce) },
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gatewayURL, client := bodyTestGateway{
				backendURL: tt.backend(t),
				timeout:    5 * time.Second,
				routeExtra: tt.routeExtra,
			}.start(t)
			if tt.warmUp {
				warmUp(t, client, gatewayURL)
			}
			start := time.Now()

			reply := upload.send(t, strings.TrimPrefix(gatewayURL, "http://"))

			assert.Equal(t, tt.expectedStatus, reply.status)
			assert.Less(t, time.Since(start), uploadTime/2, "the response was held back")
			assert.Less(t, reply.sent, upload.size, "the response was only sent once the upload ended")
			// The rest of the body is not read, so the connection cannot be reused.
			assert.True(t, reply.closes, "expected Connection: close")
			require.Error(t, reply.connErr)
			requireNoRequestBodyReader(t)
		})
	}
}

// The moment a response waits for the rest of the request body is what keeps the
// connection of the usual request, whose body is complete or about to be: giving up on
// it at once would close the connection of every request the gateway rejects.
func TestInitialize_RejectedRequestsKeepTheConnectionOfACompleteRequestBody(t *testing.T) {
	t.Run("body that arrives right behind the headers", func(t *testing.T) {
		gatewayURL, client := bodyTestGateway{
			backendURL: newBodyTestBackend(t, answerAfterTheBody),
			timeout:    5 * time.Second,
			routeExtra: oneRequestRateLimit,
		}.start(t)
		warmUp(t, client, gatewayURL)
		upload := slowUpload{size: 2, interval: 15 * time.Millisecond, keptOpen: true}

		reply := upload.send(t, strings.TrimPrefix(gatewayURL, "http://"))

		assert.Equal(t, http.StatusTooManyRequests, reply.status)
		assert.Equal(t, upload.size, reply.sent, "expected the response to wait for a body that is about to be complete")
		assert.False(t, reply.closes, "expected the connection to be kept")
	})
	t.Run("body sent with the headers", func(t *testing.T) {
		gatewayURL, client := bodyTestGateway{
			backendURL: newBodyTestBackend(t, answerAfterTheBody),
			timeout:    5 * time.Second,
			routeExtra: oneRequestRateLimit,
		}.start(t)
		warmUp(t, client, gatewayURL)
		reused := false
		trace := &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused }}

		for range 2 {
			request, err := http.NewRequestWithContext(httptrace.WithClientTrace(t.Context(), trace),
				http.MethodPost, gatewayURL+"/upload", strings.NewReader("payload"))
			require.NoError(t, err)
			res, err := client.Do(request)
			require.NoError(t, err)
			require.NoError(t, res.Body.Close())

			assert.Equal(t, http.StatusTooManyRequests, res.StatusCode)
			assert.False(t, res.Close, "expected the connection to be kept")
		}
		assert.True(t, reused, "expected the rejected requests to share their connection")
	})
}
