package gatewayhandler_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

var (
	errReadDeadline = errors.New("read deadline exceeded")
	errTestGaveUp   = errors.New("the read of the request body was never unblocked")
)

// readDeadlineRecorder is a response writer that records the read deadlines set on the
// connection of its request and whether full duplex was asked for.
type readDeadlineRecorder struct {
	*httptest.ResponseRecorder

	// expired is closed once a deadline in the past is set, which is what makes the
	// reads of a connection fail at once.
	expired    chan struct{}
	deadlines  []time.Time
	fullDuplex int
	mu         sync.Mutex
	expire     sync.Once
}

func newReadDeadlineRecorder() *readDeadlineRecorder {
	return &readDeadlineRecorder{ResponseRecorder: httptest.NewRecorder(), expired: make(chan struct{})}
}

func (r *readDeadlineRecorder) SetReadDeadline(deadline time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !deadline.IsZero() && deadline.Before(time.Now()) {
		r.expire.Do(func() { close(r.expired) })
	}
	r.deadlines = append(r.deadlines, deadline)
	return nil
}

func (r *readDeadlineRecorder) EnableFullDuplex() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fullDuplex++
	return nil
}

func (r *readDeadlineRecorder) isExpired() bool {
	select {
	case <-r.expired:
		return true
	default:
		return false
	}
}

func (r *readDeadlineRecorder) recorded() ([]time.Time, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Time(nil), r.deadlines...), r.fullDuplex
}

func newRequestBodyRoutes(timeout time.Duration, streaming bool) gateway.Routes {
	return gateway.Routes{
		{
			ID:         "r1",
			Timeout:    timeout,
			Streaming:  streaming,
			Logger:     slog.New(slog.DiscardHandler),
			Predicates: gateway.Predicates{predicate.NewMethodPredicate(http.MethodPost)},
		},
	}
}

// The response of a streaming route can start while the request body is still arriving:
// from then on the body is bounded by a deadline per read, as the response is by a
// deadline per write.
func TestGatewayHandler_ServeHTTP_RequestBodyReadDeadlines(t *testing.T) {
	const (
		timeout = time.Minute
		payload = "a request body the backend answers before it has read it"
	)
	tests := []struct {
		name            string
		expectedBody    string
		streaming       bool
		readBeforeReply bool
		expectDeadlines bool
	}{
		{
			name:            "streaming route answered while the body arrives should bound every read",
			streaming:       true,
			expectedBody:    payload,
			expectDeadlines: true,
		},
		{
			name:            "streaming route answered after the body should leave the connection alone",
			streaming:       true,
			readBeforeReply: true,
			expectedBody:    "done",
		},
		{
			name:         "regular route answered while the body arrives should keep the server read timeout",
			streaming:    false,
			expectedBody: payload,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The backend echoes the request body, unless it reads it first.
			gw := &mockGateway{
				doFunc: func(ctx *gateway.Context) error {
					reply := io.Reader(ctx.Request.BodyReader)
					if tt.readBeforeReply {
						if _, err := io.Copy(io.Discard, ctx.Request.BodyReader); err != nil {
							return fmt.Errorf("route r1 failed: %w", err)
						}
						reply = strings.NewReader("done")
					}
					ctx.Response = &gateway.Response{
						Status:     http.StatusOK,
						Headers:    http.Header{},
						BodyReader: gateway.NewReplayableBody(io.NopCloser(reply), -1),
					}
					return nil
				},
			}
			errHandler := &mockErrorHandler{
				handleFunc: func(_ *gateway.Context, err error, _ http.ResponseWriter) {
					t.Errorf("unexpected error: %v", err)
				},
			}
			handler := gatewayhandler.NewGatewayHandler(gw, newRequestBodyRoutes(timeout, tt.streaming), errHandler)
			recorder := newReadDeadlineRecorder()
			start := time.Now()

			handler.ServeHTTP(recorder,
				newTestRequest(t, http.MethodPost, "http://localhost:8080/test", strings.NewReader(payload)))

			assert.Equal(t, tt.expectedBody, recorder.Body.String())
			deadlines, fullDuplex := recorder.recorded()
			if !tt.expectDeadlines {
				assert.Empty(t, deadlines)
				assert.Zero(t, fullDuplex)
				return
			}
			assert.Equal(t, 1, fullDuplex, "expected the request body to be kept flowing")
			// One for the response and one per read, the last of which found the end
			// of the body: the deadline is taken off the connection then.
			require.GreaterOrEqual(t, len(deadlines), 3)
			last := len(deadlines) - 1
			for _, deadline := range deadlines[:last] {
				assert.False(t, deadline.Before(start.Add(timeout)), "deadline %s is too early", deadline)
				assert.False(t, deadline.After(time.Now().Add(timeout)), "deadline %s is too late", deadline)
			}
			assert.True(t, deadlines[last].IsZero(), "expected the deadline to be cleared at the end of the body")
		})
	}
}

// A request body that is still arriving when the route times out is abandoned, so that
// net/http does not read the rest of it before it writes the response.
func TestGatewayHandler_ServeHTTP_AbandonsTheRequestBodyOnTimeout(t *testing.T) {
	tests := []struct {
		gatewayErr      error
		name            string
		readBody        bool
		expectAbandoned bool
	}{
		{
			name:            "timeout with the body unread should abandon it before the error is handled",
			gatewayErr:      fmt.Errorf("route r1 failed: %w", context.DeadlineExceeded),
			expectAbandoned: true,
		},
		{
			name:       "timeout with the body read to the end should leave the connection alone",
			gatewayErr: fmt.Errorf("route r1 failed: %w", context.DeadlineExceeded),
			readBody:   true,
		},
		{
			name:       "any other error with the body unread should leave the connection alone",
			gatewayErr: gateway.ErrHTTP,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := &mockGateway{
				doFunc: func(ctx *gateway.Context) error {
					if tt.readBody {
						if _, err := io.Copy(io.Discard, ctx.Request.BodyReader); err != nil {
							return fmt.Errorf("route r1 failed: %w", err)
						}
					}
					return tt.gatewayErr
				},
			}
			recorder := newReadDeadlineRecorder()
			var onError []time.Time
			handled := false
			errHandler := &mockErrorHandler{
				handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {
					handled = true
					onError, _ = recorder.recorded()
				},
			}
			handler := gatewayhandler.NewGatewayHandler(gw, newRequestBodyRoutes(time.Minute, false), errHandler)

			handler.ServeHTTP(recorder,
				newTestRequest(t, http.MethodPost, "http://localhost:8080/test", strings.NewReader("payload")))

			require.True(t, handled, "expected the error to be handled")
			if !tt.expectAbandoned {
				assert.Empty(t, onError)
				return
			}
			require.Len(t, onError, 1)
			assert.True(t, recorder.isExpired(), "expected a read deadline in the past, actual %s", onError[0])
		})
	}
}

// blockedBody is a request body whose reads wait for the client until the read deadline
// of the connection expires, as the ones of a connection do.
type blockedBody struct {
	expired <-chan struct{}
}

func (b *blockedBody) Read([]byte) (int, error) {
	select {
	case <-b.expired:
		return 0, errReadDeadline
	case <-time.After(2 * time.Second):
		return 0, errTestGaveUp
	}
}

func (b *blockedBody) Close() error { return nil }

// bodyForwardingClient forwards the request body like an http transport does: it only
// returns once the read of the body it is waiting on has returned, and it reports the
// error of that read.
type bodyForwardingClient struct{}

func (bodyForwardingClient) Do(request *http.Request) (*http.Response, error) {
	_, err := io.Copy(io.Discard, request.Body)
	return nil, fmt.Errorf("round trip failed: %w", err)
}

// The http transport does not return before the read of the request body it is waiting
// on does: a route that times out while the client sends nothing can only answer once
// that read is made to fail, which the body of the request lets the gateway do.
func TestGatewayHandler_ServeHTTP_RouteTimeoutUnblocksTheRequestBody(t *testing.T) {
	tests := []struct {
		name      string
		streaming bool
	}{
		{name: "regular route"},
		{name: "streaming route", streaming: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var handled error
			errHandler := &mockErrorHandler{
				handleFunc: func(_ *gateway.Context, err error, _ http.ResponseWriter) { handled = err },
			}
			routes := newRequestBodyRoutes(50*time.Millisecond, tt.streaming)
			routes[0].URI = url.URL{Scheme: "http", Host: "localhost:8081"}
			handler := gatewayhandler.NewGatewayHandler(
				gateway.NewGateway(bodyForwardingClient{}), routes, errHandler)
			recorder := newReadDeadlineRecorder()
			request := newTestRequest(t, http.MethodPost, "http://localhost:8080/test", nil)
			request.Body = &blockedBody{expired: recorder.expired}
			request.ContentLength = -1
			start := time.Now()

			handler.ServeHTTP(recorder, request)

			require.ErrorIs(t, handled, context.DeadlineExceeded, "expected the route timeout to be reported")
			assert.Less(t, time.Since(start), time.Second)
		})
	}
}

// An http transport closes the request body from its own goroutine, which may still be
// at it when the handler returns. The connection belongs to net/http from then on: the
// handler releases the body itself, so that nothing sets a deadline on it anymore.
func TestGatewayHandler_ServeHTTP_ReleasesTheRequestBody(t *testing.T) {
	closing := make(chan struct{})
	handlerDone := make(chan struct{})
	closed := make(chan struct{})
	gw := &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			body := ctx.Request.BodyReader
			// The close of the body is held until the handler has returned.
			body.ObserveStream(nil, func(int64, error) {
				close(closing)
				<-handlerDone
			})
			go func() {
				defer close(closed)
				_ = body.Close()
			}()
			<-closing
			ctx.Response = &gateway.Response{
				Status:     http.StatusOK,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(strings.NewReader("done")), 4),
			}
			return nil
		},
	}
	errHandler := &mockErrorHandler{
		handleFunc: func(_ *gateway.Context, err error, _ http.ResponseWriter) {
			t.Errorf("unexpected error: %v", err)
		},
	}
	handler := gatewayhandler.NewGatewayHandler(gw, newRequestBodyRoutes(time.Minute, false), errHandler)
	recorder := newReadDeadlineRecorder()
	request := newTestRequest(t, http.MethodPost, "http://localhost:8080/test", strings.NewReader("payload"))

	handler.ServeHTTP(recorder, request)
	body, abandonable := request.Body.(interface{ Abandon() })
	require.True(t, abandonable, "expected the request body to be the one the handler tracks")
	body.Abandon()
	close(handlerDone)
	<-closed

	deadlines, _ := recorder.recorded()
	assert.Empty(t, deadlines, "expected no read deadline once the handler has returned")
}
