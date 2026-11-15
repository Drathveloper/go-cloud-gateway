package gateway_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// slowPostProcessFilter takes a fixed time to post-process the response.
type slowPostProcessFilter struct {
	delay time.Duration
}

func (f *slowPostProcessFilter) PreProcess(_ *gateway.Context) error { return nil }

func (f *slowPostProcessFilter) PostProcess(_ *gateway.Context) error {
	time.Sleep(f.delay)
	return nil
}

func (f *slowPostProcessFilter) Name() string { return "SlowPostProcess" }

type closeTrackingBody struct {
	io.Reader

	closed bool
}

func (b *closeTrackingBody) Close() error {
	b.closed = true
	return nil
}

func newStreamingTestRequest(t *testing.T) *gateway.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/stream", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	return gateway.NewGatewayRequest(request)
}

func TestGateway_Do_RouteTimeoutAfterResponse(t *testing.T) {
	tests := []struct {
		name          string
		streaming     bool
		expectExpired bool
	}{
		{
			name:          "streaming route timeout should stop once the response is ready",
			streaming:     true,
			expectExpired: false,
		},
		{
			name:          "regular route timeout should keep bounding the whole exchange",
			streaming:     false,
			expectExpired: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				route := &gateway.Route{ID: "r1", Timeout: time.Second, Streaming: tt.streaming}
				ctx, cancel := gateway.NewGatewayContext(t.Context(), route, newStreamingTestRequest(t))
				defer cancel()
				gw := gateway.NewGateway(&MockHTTPClient{
					Response: &http.Response{
						StatusCode:    http.StatusOK,
						ContentLength: -1,
						Body:          io.NopCloser(strings.NewReader("stream")),
					},
				})

				if err := gw.Do(ctx); err != nil {
					t.Fatalf("expected nil err actual %v", err)
				}
				time.Sleep(time.Minute)
				synctest.Wait()

				if expired := ctx.Err() != nil; expired != tt.expectExpired {
					t.Errorf("expected context expired %t after the route timeout, actual error %v",
						tt.expectExpired, ctx.Err())
				}
			})
		})
	}
}

func TestGateway_Do_StreamingRouteTimeoutCoversPostProcess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		route := &gateway.Route{
			ID:        "r1",
			Timeout:   time.Second,
			Streaming: true,
			Filters:   gateway.Filters{&slowPostProcessFilter{delay: time.Minute}},
		}
		ctx, cancel := gateway.NewGatewayContext(t.Context(), route, newStreamingTestRequest(t))
		defer cancel()
		body := &closeTrackingBody{Reader: strings.NewReader("stream")}
		gw := gateway.NewGateway(&MockHTTPClient{
			Response: &http.Response{StatusCode: http.StatusOK, ContentLength: -1, Body: body},
		})

		err := gw.Do(ctx)

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded actual %v", err)
		}
		if !body.closed {
			t.Error("expected the backend response body to be closed")
		}
	})
}

// ctxErrHTTPClient waits for the request context to be done and returns its Err, like
// the HTTP/2 transport of net/http does, rather than its cause.
type ctxErrHTTPClient struct{}

func (c *ctxErrHTTPClient) Do(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, fmt.Errorf("http2: %w", req.Context().Err())
}

// ctxErrPreProcessFilter waits for the route context to be done and returns its Err.
type ctxErrPreProcessFilter struct {
	post bool
}

func (f *ctxErrPreProcessFilter) PreProcess(ctx *gateway.Context) error {
	if f.post {
		return nil
	}
	<-ctx.Done()
	return fmt.Errorf("filter: %w", ctx.Err())
}

func (f *ctxErrPreProcessFilter) PostProcess(ctx *gateway.Context) error {
	if !f.post {
		return nil
	}
	<-ctx.Done()
	return fmt.Errorf("filter: %w", ctx.Err())
}

func (f *ctxErrPreProcessFilter) Name() string { return "CtxErr" }

// The context of a streaming route is cancelled with the timeout as its cause, but its
// Err is context.Canceled: what reports ctx.Err() must still be answered as a timeout,
// not as a client that went away.
func TestGateway_Do_StreamingRouteTimeoutReportedAsCanceled(t *testing.T) {
	tests := []struct {
		client  gateway.HTTPClient
		name    string
		filters gateway.Filters
	}{
		{name: "http client", client: &ctxErrHTTPClient{}},
		{name: "pre-process filter", client: &MockHTTPClient{}, filters: gateway.Filters{&ctxErrPreProcessFilter{}}},
		{
			name: "post-process filter",
			client: &MockHTTPClient{Response: &http.Response{
				StatusCode: http.StatusOK, ContentLength: -1, Body: io.NopCloser(strings.NewReader("")),
			}},
			filters: gateway.Filters{&ctxErrPreProcessFilter{post: true}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				route := &gateway.Route{ID: "r1", Timeout: time.Second, Streaming: true, Filters: tt.filters}
				ctx, cancel := gateway.NewGatewayContext(t.Context(), route, newStreamingTestRequest(t))
				defer cancel()

				err := gateway.NewGateway(tt.client).Do(ctx)

				if !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("expected context.DeadlineExceeded actual %v", err)
				}
				if errors.Is(err, context.Canceled) {
					t.Errorf("expected the error not to be a context.Canceled actual %v", err)
				}
			})
		})
	}
}

// A streaming route whose client goes away is still reported as context.Canceled.
func TestGateway_Do_StreamingRouteClientGoneStaysCanceled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		parent, cancelParent := context.WithCancel(t.Context())
		route := &gateway.Route{ID: "r1", Timeout: time.Minute, Streaming: true}
		ctx, cancel := gateway.NewGatewayContext(parent, route, newStreamingTestRequest(t))
		defer cancel()
		go func() {
			time.Sleep(time.Second)
			cancelParent()
		}()

		err := gateway.NewGateway(&ctxErrHTTPClient{}).Do(ctx)

		if !errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.Canceled only, actual %v", err)
		}
	})
}
