package gateway_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/circuitbreaker"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

type MockHTTPClient struct {
	Response *http.Response
	Err      error
}

func (c *MockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	return c.Response, c.Err
}

func newClosedReplayableBody() *gateway.ReplayableBody {
	rb := gateway.NewReplayableBody(nil, 0)
	_ = rb.Close()
	return rb
}

func TestGateway_Do(t *testing.T) {
	tests := []struct {
		httpClient       gateway.HTTPClient
		expectedErr      error
		route            *gateway.Route
		request          *gateway.Request
		expectedResponse *gateway.Response
		name             string
		expectedErrMsg   string
		globalFilters    gateway.Filters
	}{
		{
			name: "Do gateway should succeed",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: &http.Response{
					ContentLength: 0,
					StatusCode:    http.StatusOK,
				},
				Err: nil,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:     http.MethodPost,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer([]byte("someBody"))), int64(len("someBody"))),
			},
			expectedResponse: &gateway.Response{
				Status:     http.StatusOK,
				BodyReader: gateway.NewReplayableBody(nil, 0),
			},
			expectedErr: nil,
		},
		{
			name: "Do gateway should succeed when request body is empty",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: &http.Response{
					StatusCode: http.StatusOK,
				},
				Err: nil,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:     http.MethodPost,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer([]byte{})), int64(0)),
			},
			expectedResponse: &gateway.Response{
				Status:     http.StatusOK,
				BodyReader: gateway.NewReplayableBody(nil, 0),
			},
			expectedErr: nil,
		},
		{
			name: "Do gateway should return error when preprocess filter failed",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: &http.Response{
					StatusCode: http.StatusOK,
				},
				Err: nil,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  io.EOF,
						PostProcessErr: nil,
						ID:             "F1",
					},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:     http.MethodPost,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer([]byte("someBody"))), int64(len("someBody"))),
			},
			expectedResponse: nil,
			expectedErr:      io.EOF,
			expectedErrMsg:   "gateway request for route r1 failed: pre-process filters failed with filter F1: EOF",
		},
		{
			name: "Do gateway should return error when deadline exceeded",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      context.DeadlineExceeded,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:     http.MethodPost,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer([]byte("someBody"))), int64(len("someBody"))),
			},
			expectedResponse: nil,
			expectedErr:      context.DeadlineExceeded,
			expectedErrMsg:   "gateway request for route r1 failed: context deadline exceeded",
		},
		{
			name: "Do gateway should return error when context cancelled",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      context.Canceled,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:     http.MethodPost,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer([]byte("someBody"))), int64(len("someBody"))),
			},
			expectedResponse: nil,
			expectedErr:      context.Canceled,
			expectedErrMsg:   "gateway request for route r1 failed: context canceled",
		},
		{
			name: "Do gateway should return error when generic http error",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      errors.New("someErr"),
			},
			route: &gateway.Route{
				ID: "r1",
				URI: url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:     http.MethodPost,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer([]byte("someBody"))), int64(len("someBody"))),
			},
			expectedResponse: nil,
			expectedErr:      gateway.ErrHTTP,
			expectedErrMsg:   "gateway request for route r1 failed: gateway http request to backend failed: someErr",
		},
		{
			name: "Do gateway should return error when generic circuit breaker is open",
			globalFilters: gateway.Filters{
				&DummyFilter{
					ID: "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      circuitbreaker.ErrOpenState,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:     http.MethodPost,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer([]byte("someBody"))), int64(len("someBody"))),
			},
			expectedResponse: nil,
			expectedErr:      gateway.ErrCircuitBreaker,
			expectedErrMsg:   "gateway request for route r1 failed: circuit breaker failed: circuit breaker is open",
		},
		{
			name: "Do gateway should return error when generic circuit breaker is half open",
			globalFilters: gateway.Filters{
				&DummyFilter{
					ID: "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      circuitbreaker.ErrHalfOpenRequestExceeded,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:     http.MethodPost,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer([]byte("someBody"))), int64(len("someBody"))),
			},
			expectedResponse: nil,
			expectedErr:      gateway.ErrCircuitBreaker,
			expectedErrMsg:   "gateway request for route r1 failed: circuit breaker failed: too many requests while circuit breaker is half-open",
		},
		{
			name: "Do gateway should return error when post process filters failed",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: &http.Response{
					StatusCode: http.StatusOK,
				},
				Err: nil,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: io.EOF,
						ID:             "F1",
					},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:     http.MethodPost,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer([]byte("someBody"))), int64(len("someBody"))),
			},
			expectedResponse: &gateway.Response{
				Status:     http.StatusOK,
				BodyReader: newClosedReplayableBody(),
			},
			expectedErr:    io.EOF,
			expectedErrMsg: "gateway request for route r1 failed: post-process filters failed with filter F1: EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := gateway.NewGateway(tt.httpClient)
			ctx, _ := gateway.NewGatewayContext(t.Context(), tt.route, tt.request)

			err := gw.Do(ctx)

			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err != nil && tt.expectedErrMsg != err.Error() {
				t.Errorf("expected err msg %s actual %s", tt.expectedErrMsg, err)
			}
			if !reflect.DeepEqual(tt.expectedResponse, ctx.Response) {
				t.Errorf("expected response %v actual %v", tt.expectedResponse, ctx.Response)
			}
		})
	}
}

type captureHTTPClient struct {
	captured *http.Request
	response *http.Response
}

func (c *captureHTTPClient) Do(r *http.Request) (*http.Response, error) {
	c.captured = r
	return c.response, nil
}

func TestGateway_Do_EmptyBodyIsNilForTransportRetries(t *testing.T) {
	tests := []struct {
		body       *gateway.ReplayableBody
		name       string
		wantLength int64
		wantNil    bool
	}{
		{
			name:       "declared-empty body is sent as nil body",
			body:       gateway.NewReplayableBody(nil, 0),
			wantNil:    true,
			wantLength: 0,
		},
		{
			name:       "non-empty body is forwarded",
			body:       gateway.NewReplayableBody(io.NopCloser(bytes.NewReader([]byte("someBody"))), int64(len("someBody"))),
			wantNil:    false,
			wantLength: int64(len("someBody")),
		},
		{
			name:       "unknown length body is forwarded",
			body:       gateway.NewReplayableBody(io.NopCloser(bytes.NewReader([]byte("someBody"))), -1),
			wantNil:    false,
			wantLength: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &captureHTTPClient{response: &http.Response{StatusCode: http.StatusOK}}
			route := &gateway.Route{
				ID:      "r1",
				URI:     url.URL{Scheme: "https", Host: "example.org"},
				Timeout: time.Minute,
			}
			request := &gateway.Request{
				URL:        &url.URL{Scheme: "https", Host: "example.org", Path: "/test"},
				Method:     http.MethodGet,
				Headers:    http.Header{},
				BodyReader: tt.body,
			}
			gw := gateway.NewGateway(client)
			ctx, cancel := gateway.NewGatewayContext(t.Context(), route, request)
			defer cancel()

			if err := gw.Do(ctx); err != nil {
				t.Fatalf("Do failed: %v", err)
			}
			if gotNil := client.captured.Body == nil; gotNil != tt.wantNil {
				t.Errorf("expected backend request body nil=%v, actual nil=%v", tt.wantNil, gotNil)
			}
			if client.captured.ContentLength != tt.wantLength {
				t.Errorf("expected content length %d, actual %d", tt.wantLength, client.captured.ContentLength)
			}
		})
	}
}

func TestGateway_Do_StripsHopByHopRequestHeaders(t *testing.T) {
	client := &captureHTTPClient{response: &http.Response{StatusCode: http.StatusOK}}
	route := &gateway.Route{
		ID:      "r1",
		URI:     url.URL{Scheme: "https", Host: "example.org"},
		Timeout: time.Minute,
	}
	request := &gateway.Request{
		URL:    &url.URL{Scheme: "https", Host: "example.org", Path: "/test"},
		Method: http.MethodGet,
		Headers: http.Header{
			"Connection":    {"close, X-Custom-Hop"},
			"X-Custom-Hop":  {"value"},
			"Keep-Alive":    {"timeout=5"},
			"Upgrade":       {"websocket"},
			"Content-Type":  {"application/json"},
			"Authorization": {"Bearer token"},
		},
		BodyReader: gateway.NewReplayableBody(nil, 0),
	}
	gw := gateway.NewGateway(client)
	ctx, cancel := gateway.NewGatewayContext(t.Context(), route, request)
	defer cancel()

	if err := gw.Do(ctx); err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	for _, name := range []string{"Connection", "X-Custom-Hop", "Keep-Alive", "Upgrade"} {
		if got := client.captured.Header.Get(name); got != "" {
			t.Errorf("expected hop-by-hop header %s stripped from backend request, actual %q", name, got)
		}
	}
	for name, want := range map[string]string{"Content-Type": "application/json", "Authorization": "Bearer token"} {
		if got := client.captured.Header.Get(name); got != want {
			t.Errorf("expected end-to-end header %s=%q kept, actual %q", name, want, got)
		}
	}
}

func TestGateway_Do_ClosesBackendBodyOnPostProcessError(t *testing.T) {
	backendBody := &closeCountingBody{Reader: bytes.NewReader([]byte("backend response"))}
	httpClient := &MockHTTPClient{
		Response: &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: int64(len("backend response")),
			Body:          backendBody,
		},
	}
	route := &gateway.Route{
		ID:  "r1",
		URI: url.URL{Scheme: "https", Host: "example.org"},
		Filters: gateway.Filters{
			&DummyFilter{
				PostProcessErr: io.EOF,
				ID:             "F1",
			},
		},
	}
	request := &gateway.Request{
		URL:        &url.URL{Scheme: "https", Host: "example.org", Path: "/server/test"},
		Method:     http.MethodGet,
		Headers:    http.Header{},
		BodyReader: gateway.NewReplayableBody(nil, 0),
	}
	gw := gateway.NewGateway(httpClient)
	ctx, cancel := gateway.NewGatewayContext(t.Context(), route, request)
	defer cancel()

	if err := gw.Do(ctx); err == nil {
		t.Fatal("expected error from post-process filter")
	}
	if backendBody.closes != 1 {
		t.Errorf("expected backend body closed once, actual %d", backendBody.closes)
	}
}
