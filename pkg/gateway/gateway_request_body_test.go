package gateway_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

var errBodyOfAGoneClient = errors.New("client gone")

// waitingBody is the body of a request whose client sends nothing: its reads wait until
// the body is abandoned or the client goes away.
type waitingBody struct {
	abandoned chan struct{}
	gone      chan struct{}
	abandon   sync.Once
}

func newWaitingBody() *waitingBody {
	return &waitingBody{abandoned: make(chan struct{}), gone: make(chan struct{})}
}

func (b *waitingBody) Read([]byte) (int, error) {
	select {
	case <-b.abandoned:
		return 0, fmt.Errorf("request body abandoned: %w", context.DeadlineExceeded)
	case <-b.gone:
		return 0, errBodyOfAGoneClient
	}
}

func (b *waitingBody) Close() error { return nil }

func (b *waitingBody) Abandon() {
	b.abandon.Do(func() { close(b.abandoned) })
}

func (b *waitingBody) isAbandoned() bool {
	select {
	case <-b.abandoned:
		return true
	default:
		return false
	}
}

// bodyForwardingClient forwards the request body like an http transport does: it only
// returns once the read of the body it is waiting on has returned, and it reports the
// cancellation of the request rather than the failure of that read.
type bodyForwardingClient struct{}

func (bodyForwardingClient) Do(request *http.Request) (*http.Response, error) {
	_, err := io.Copy(io.Discard, request.Body)
	if cause := context.Cause(request.Context()); cause != nil {
		return nil, fmt.Errorf("round trip cancelled: %w", cause)
	}
	return nil, fmt.Errorf("round trip failed: %w", err)
}

func newWaitingRequest(t *testing.T, body io.ReadCloser) *gateway.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost/upload", body)
	require.NoError(t, err)
	request.ContentLength = -1
	return gateway.NewGatewayRequest(request)
}

// An http transport does not return before the read of the request body it is waiting
// on does: the timeout of a route whose client sends nothing would only be reported
// when that read gives up, which is up to the server read timeout.
func TestGateway_Do_AbandonsTheRequestBodyOnRouteTimeout(t *testing.T) {
	const timeout = time.Minute
	tests := []struct {
		name      string
		streaming bool
		observed  bool
	}{
		{name: "regular route"},
		{name: "streaming route", streaming: true},
		{name: "body wrapped by a stream observer", observed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				body := newWaitingBody()
				request := newWaitingRequest(t, body)
				if tt.observed {
					request.BodyReader.ObserveStream(func([]byte) {}, nil)
				}
				route := &gateway.Route{ID: "r1", Timeout: timeout, Streaming: tt.streaming}
				ctx, cancel := gateway.NewGatewayContext(t.Context(), route, request)
				defer cancel()
				start := time.Now()

				err := gateway.NewGateway(bodyForwardingClient{}).Do(ctx)

				require.ErrorIs(t, err, context.DeadlineExceeded)
				assert.Equal(t, timeout, time.Since(start))
				assert.True(t, body.isAbandoned())
			})
		})
	}
}

// A client that goes away makes the reads of its body fail by itself.
func TestGateway_Do_KeepsTheRequestBodyOfAGoneClient(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		body := newWaitingBody()
		parent, clientGone := context.WithCancel(t.Context())
		route := &gateway.Route{ID: "r1", Timeout: time.Minute}
		ctx, cancel := gateway.NewGatewayContext(parent, route, newWaitingRequest(t, body))
		defer cancel()
		go func() {
			time.Sleep(time.Second)
			clientGone()
			close(body.gone)
		}()

		err := gateway.NewGateway(bodyForwardingClient{}).Do(ctx)
		synctest.Wait()

		require.ErrorIs(t, err, context.Canceled)
		assert.False(t, body.isAbandoned())
	})
}

// Once Do has returned the body is not its business anymore: the deadline of a regular
// route, which keeps running while the response is sent, must not abandon it.
func TestGateway_Do_LeavesTheRequestBodyAloneOnceItReturns(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		body := newWaitingBody()
		route := &gateway.Route{ID: "r1", Timeout: time.Minute}
		ctx, cancel := gateway.NewGatewayContext(t.Context(), route, newWaitingRequest(t, body))
		defer cancel()
		gw := gateway.NewGateway(&MockHTTPClient{
			Response: &http.Response{
				StatusCode:    http.StatusOK,
				ContentLength: -1,
				Body:          io.NopCloser(strings.NewReader("answered before the body was read")),
			},
		})

		require.NoError(t, gw.Do(ctx))
		time.Sleep(time.Hour)
		synctest.Wait()

		require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
		assert.False(t, body.isAbandoned())
	})
}
