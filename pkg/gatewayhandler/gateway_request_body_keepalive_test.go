package gatewayhandler_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
)

// heldEndBody holds back the end of a request body: the read that reaches it only
// returns after hold. net/http has seen the end of the body by then, which is the moment
// a route timeout must not find the body still open.
type heldEndBody struct {
	io.ReadCloser

	hold time.Duration
}

func (b *heldEndBody) Read(output []byte) (int, error) {
	read, err := b.ReadCloser.Read(output)
	if errors.Is(err, io.EOF) {
		time.Sleep(b.hold)
	}
	return read, err //nolint:wrapcheck
}

// firstCallSlowClient forwards the request body and then leaves its first request
// unanswered until the route gives up. It answers the next ones at once, unless their
// context is done already, which an http transport does not get past either.
type firstCallSlowClient struct {
	calls atomic.Int32
}

func (c *firstCallSlowClient) Do(request *http.Request) (*http.Response, error) {
	if err := request.Context().Err(); err != nil {
		return nil, fmt.Errorf("round trip failed: %w", err)
	}
	if _, err := io.Copy(io.Discard, request.Body); err != nil {
		return nil, fmt.Errorf("round trip failed: %w", err)
	}
	if c.calls.Add(1) == 1 {
		<-request.Context().Done()
		return nil, fmt.Errorf("round trip failed: %w", context.Cause(request.Context()))
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{},
		Body:          io.NopCloser(strings.NewReader("ok")),
		ContentLength: int64(len("ok")),
	}, nil
}

// postOnce sends a request with a body and reports the status of its response, whether
// the response announced that the connection is closed and whether the request went over
// a connection that had been used before.
func postOnce(tb testing.TB, client *http.Client, target string) (int, bool, bool) {
	tb.Helper()
	reused := false
	trace := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused },
	}
	request, err := http.NewRequestWithContext(
		httptrace.WithClientTrace(tb.Context(), trace), http.MethodPost, target, strings.NewReader("hello"))
	require.NoError(tb, err)
	response, err := client.Do(request)
	require.NoError(tb, err)
	defer response.Body.Close() //nolint:errcheck
	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(tb, err)
	return response.StatusCode, response.Close, reused
}

// A route can time out at the very moment the last read of the request body returns.
// net/http is then already watching the connection for a client that goes away, the
// abandoned body makes that watch fail, and net/http cancels the context of the
// connection without closing it: kept alive, every later request of that connection
// would be answered as if its client had gone. The connection is closed instead.
func TestGatewayHandler_ServeHTTP_ClosesTheConnectionOfAnAbandonedRequestBody(t *testing.T) {
	routes := newRequestBodyRoutes(30*time.Millisecond, false)
	routes[0].URI = url.URL{Scheme: "http", Host: "localhost:8081"}
	handler := gatewayhandler.NewGatewayHandler(
		gateway.NewGateway(&firstCallSlowClient{}), routes, gatewayhandler.BaseErrorHandler())
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if requests.Add(1) == 1 {
			// The route times out while the end of the body is on its way back.
			request.Body = &heldEndBody{ReadCloser: request.Body, hold: 150 * time.Millisecond}
		}
		handler.ServeHTTP(writer, request)
	}))
	defer server.Close()
	// One connection at most, so that the second request can only avoid the first
	// one's connection when the server has closed it.
	transport := &http.Transport{MaxConnsPerHost: 1}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}

	status, closing, _ := postOnce(t, client, server.URL+"/test")
	assert.Equal(t, http.StatusGatewayTimeout, status)
	assert.True(t, closing, "expected the response to announce that the connection is closed")

	status, closing, reused := postOnce(t, client, server.URL+"/test")
	assert.Equal(t, http.StatusOK, status)
	assert.False(t, reused, "expected the connection of the abandoned body not to be used again")
	assert.False(t, closing, "expected a request that is not abandoned to keep its connection")

	status, _, reused = postOnce(t, client, server.URL+"/test")
	assert.Equal(t, http.StatusOK, status)
	assert.True(t, reused, "expected the connection of a request that is not abandoned to be kept")
}

// The connection of an abandoned body is only closed on HTTP/1: an HTTP/2 stream has
// none of its own, and the header would shut down the connection of every other stream.
func TestGatewayHandler_ServeHTTP_AbandonedRequestBodyClosesOnlyHTTP1Connections(t *testing.T) {
	tests := []struct {
		gatewayErr         error
		name               string
		expectedConnection string
		protoMajor         int
	}{
		{
			name:               "timeout on http/1 with the body unread should close the connection",
			gatewayErr:         fmt.Errorf("route r1 failed: %w", context.DeadlineExceeded),
			protoMajor:         1,
			expectedConnection: "close",
		},
		{
			name:       "timeout on http/2 with the body unread should keep the connection",
			gatewayErr: fmt.Errorf("route r1 failed: %w", context.DeadlineExceeded),
			protoMajor: 2,
		},
		{
			name:       "any other error on http/1 with the body unread should keep the connection",
			gatewayErr: gateway.ErrHTTP,
			protoMajor: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := &mockGateway{doFunc: func(*gateway.Context) error { return tt.gatewayErr }}
			connection := "not handled"
			errHandler := &mockErrorHandler{
				handleFunc: func(_ *gateway.Context, _ error, writer http.ResponseWriter) {
					connection = writer.Header().Get("Connection")
				},
			}
			handler := gatewayhandler.NewGatewayHandler(gw, newRequestBodyRoutes(time.Minute, false), errHandler)
			request := newTestRequest(t, http.MethodPost, "http://localhost:8080/test", strings.NewReader("payload"))
			request.ProtoMajor = tt.protoMajor

			handler.ServeHTTP(newReadDeadlineRecorder(), request)

			assert.Equal(t, tt.expectedConnection, connection)
		})
	}
}
