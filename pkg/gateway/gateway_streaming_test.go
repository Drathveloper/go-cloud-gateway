package gateway_test

import (
	"context"
	"errors"
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
