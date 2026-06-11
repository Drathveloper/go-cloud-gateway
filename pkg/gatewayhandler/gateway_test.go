package gatewayhandler_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

type mockGateway struct {
	doFunc func(ctx *gateway.Context) error
}

func (m *mockGateway) Do(ctx *gateway.Context) error {
	return m.doFunc(ctx)
}

type mockErrorHandler struct {
	handleFunc func(ctx *gateway.Context, err error, w http.ResponseWriter)
}

func (m *mockErrorHandler) Handle(ctx *gateway.Context, err error, w http.ResponseWriter) {
	m.handleFunc(ctx, err, w)
}

func newTestRequest(tb testing.TB, method, target string, body io.Reader) *http.Request {
	tb.Helper()
	req, err := http.NewRequestWithContext(tb.Context(), method, target, body)
	if err != nil {
		tb.Fatalf("failed to build request: %v", err)
	}
	return req
}

func TestGatewayHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		gatewayErr       error
		expectedErr      error
		request          *http.Request
		expectedResponse gateway.Response
		expectedHeaders  http.Header
		name             string
		expectedBody     string
		routes           gateway.Routes
	}{
		{
			name: "serve HTTP should succeed when request is valid and route is found",
			routes: gateway.Routes{
				{
					ID: "r1",
					Predicates: gateway.Predicates{
						predicate.NewMethodPredicate(http.MethodGet),
					},
				},
			},
			request: newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil),
			expectedResponse: gateway.Response{
				Headers: map[string][]string{
					"Content-Length": {"4"},
				},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBufferString("test")), int64(4)),
				Status:     http.StatusOK,
			},
			expectedHeaders: http.Header{
				"Content-Length": {"4"},
			},
			expectedBody: "test",
			gatewayErr:   nil,
		},
		{
			name: "serve HTTP should succeed when request is valid and response doesn't have content length",
			routes: gateway.Routes{
				{
					ID: "r1",
					Predicates: gateway.Predicates{
						predicate.NewMethodPredicate(http.MethodGet),
					},
				},
			},
			request: newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil),
			expectedResponse: gateway.Response{
				Headers: map[string][]string{
					"Transfer-Encoding": {"chunked"},
				},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBufferString("test")), int64(-1)),
				Status:     http.StatusOK,
			},
			// the transfer encoding is the server's choice: the handler must not
			// set it by hand, and the backend hop-by-hop header must not leak.
			expectedHeaders: http.Header{},
			expectedBody:    "test",
			gatewayErr:      nil,
		},
		{
			name: "serve HTTP should handle error when request is valid and route isn't found",
			routes: gateway.Routes{
				{
					ID: "r1",
					Predicates: gateway.Predicates{
						predicate.NewMethodPredicate(http.MethodPost),
					},
				},
			},
			request:     newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil),
			expectedErr: gatewayhandler.ErrRouteNotFound,
		},
		{
			name: "serve HTTP should handle error when gateway request failed",
			routes: gateway.Routes{
				{
					ID: "r1",
					Predicates: gateway.Predicates{
						predicate.NewMethodPredicate(http.MethodGet),
					},
				},
			},
			request:     newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil),
			gatewayErr:  gateway.ErrHTTP,
			expectedErr: gateway.ErrHTTP,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			gw := &mockGateway{
				doFunc: func(ctx *gateway.Context) error {
					// writeResponse mutates the response headers in place: hand the
					// handler a copy so the table expectation stays pristine.
					res := tt.expectedResponse
					res.Headers = tt.expectedResponse.Headers.Clone()
					ctx.Response = &res
					return tt.gatewayErr
				},
			}
			errHandler := &mockErrorHandler{
				handleFunc: func(_ *gateway.Context, err error, _ http.ResponseWriter) {
					if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
						t.Errorf("expected err %s actual %s", tt.expectedErr, err)
					}
				},
			}
			gwHandler := gatewayhandler.NewGatewayHandler(gw, tt.routes, errHandler)

			gwHandler.ServeHTTP(recorder, tt.request)

			if tt.expectedErr == nil && recorder.Code != tt.expectedResponse.Status {
				t.Errorf("expected status %d actual %d", tt.expectedResponse.Status, recorder.Code)
			}
			if tt.expectedErr == nil && recorder.Body.String() != tt.expectedBody {
				t.Errorf("expected body %s actual %s", tt.expectedBody, recorder.Body.String())
			}
			if tt.expectedErr == nil && !reflect.DeepEqual(recorder.Header(), tt.expectedHeaders) {
				t.Errorf("expected headers %v actual %v", tt.expectedHeaders, recorder.Header())
			}
		})
	}
}

type closeCountingBody struct {
	io.Reader

	closes int
}

func (c *closeCountingBody) Close() error {
	c.closes++
	return nil
}

func TestGatewayHandler_ServeHTTP_ClosesResponseBodyOnError(t *testing.T) {
	body := &closeCountingBody{Reader: bytes.NewReader([]byte("partial backend response"))}
	gw := &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			ctx.Response = &gateway.Response{
				Status:     http.StatusOK,
				BodyReader: gateway.NewReplayableBody(body, int64(len("partial backend response"))),
			}
			return gateway.ErrHTTP
		},
	}
	errHandler := &mockErrorHandler{
		handleFunc: func(_ *gateway.Context, _ error, w http.ResponseWriter) {
			http.Error(w, "", http.StatusBadGateway)
		},
	}
	routes := gateway.Routes{
		{
			ID: "r1",
			Predicates: gateway.Predicates{
				predicate.NewMethodPredicate(http.MethodGet),
			},
		},
	}
	gwHandler := gatewayhandler.NewGatewayHandler(gw, routes, errHandler)
	recorder := httptest.NewRecorder()

	request, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost:8080/test", nil)
	gwHandler.ServeHTTP(recorder, request)

	if body.closes != 1 {
		t.Errorf("expected response body closed once, actual %d", body.closes)
	}
}

func TestGatewayHandler_ServeHTTP_StripsHopByHopResponseHeaders(t *testing.T) {
	gw := &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			ctx.Response = &gateway.Response{
				Status: http.StatusOK,
				Headers: http.Header{
					"Connection":   {"close"},
					"Keep-Alive":   {"timeout=5"},
					"Content-Type": {"application/json"},
				},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBufferString("test")), int64(4)),
			}
			return nil
		},
	}
	errHandler := &mockErrorHandler{
		handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {},
	}
	routes := gateway.Routes{
		{
			ID:      "r1",
			Timeout: time.Minute,
			Predicates: gateway.Predicates{
				predicate.NewMethodPredicate(http.MethodGet),
			},
		},
	}
	gwHandler := gatewayhandler.NewGatewayHandler(gw, routes, errHandler)
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost:8080/test", nil)

	gwHandler.ServeHTTP(recorder, request)

	for _, name := range []string{"Connection", "Keep-Alive"} {
		if got := recorder.Header().Get(name); got != "" {
			t.Errorf("expected hop-by-hop header %s stripped from client response, actual %q", name, got)
		}
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("expected end-to-end header Content-Type kept, actual %q", got)
	}
	if recorder.Body.String() != "test" {
		t.Errorf("expected body %q, actual %q", "test", recorder.Body.String())
	}
}

func TestGatewayHandler_ServeHTTP_StaleBackendContentLengthIsOverridden(t *testing.T) {
	gw := &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			// A backend Content-Length that no longer matches the body, as left
			// behind by a filter that buffered and modified the response.
			ctx.Response = &gateway.Response{
				Status: http.StatusOK,
				Headers: http.Header{
					"Content-Length": {"999"},
				},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBufferString("test")), int64(4)),
			}
			return nil
		},
	}
	errHandler := &mockErrorHandler{
		handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {},
	}
	routes := gateway.Routes{
		{
			ID:      "r1",
			Timeout: time.Minute,
			Predicates: gateway.Predicates{
				predicate.NewMethodPredicate(http.MethodGet),
			},
		},
	}
	gwHandler := gatewayhandler.NewGatewayHandler(gw, routes, errHandler)
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost:8080/test", nil)

	gwHandler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Content-Length"); got != "4" {
		t.Errorf("expected the body length to override the stale backend Content-Length, actual %q", got)
	}
}

func TestGatewayHandler_ServeHTTP_EndToEndTransferEncoding(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		bodyLen     int64
		wantChunked bool
	}{
		{
			name:        "known length is sent with content length",
			payload:     "streamed body",
			bodyLen:     int64(len("streamed body")),
			wantChunked: false,
		},
		{
			// the payload must overflow the server output buffer: smaller
			// unknown-length bodies get a server-computed Content-Length instead.
			name:        "unknown length streams chunked",
			payload:     strings.Repeat("a", 128*1024),
			bodyLen:     -1,
			wantChunked: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := &mockGateway{
				doFunc: func(ctx *gateway.Context) error {
					ctx.Response = &gateway.Response{
						Status:     http.StatusOK,
						Headers:    http.Header{},
						BodyReader: gateway.NewReplayableBody(io.NopCloser(strings.NewReader(tt.payload)), tt.bodyLen),
					}
					return nil
				},
			}
			errHandler := &mockErrorHandler{
				handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {},
			}
			routes := gateway.Routes{
				{
					ID:      "r1",
					Timeout: time.Minute,
					Predicates: gateway.Predicates{
						predicate.NewMethodPredicate(http.MethodGet),
					},
				},
			}
			server := httptest.NewServer(gatewayhandler.NewGatewayHandler(gw, routes, errHandler))
			defer server.Close()

			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/test", nil)
			if err != nil {
				t.Fatalf("failed to build request: %v", err)
			}
			res, err := server.Client().Do(request)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer res.Body.Close() //nolint:errcheck
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("reading body failed: %v", err)
			}
			if string(body) != tt.payload {
				t.Errorf("expected body of %d bytes, actual %d bytes", len(tt.payload), len(body))
			}
			gotChunked := slices.Contains(res.TransferEncoding, "chunked")
			if gotChunked != tt.wantChunked {
				t.Errorf("expected chunked=%v, actual transfer encoding %v", tt.wantChunked, res.TransferEncoding)
			}
			if !tt.wantChunked && res.ContentLength != int64(len(tt.payload)) {
				t.Errorf("expected content length %d, actual %d", len(tt.payload), res.ContentLength)
			}
		})
	}
}

func TestGatewayHandler_ServeHTTP_AbortsConnectionOnBodyCopyError(t *testing.T) {
	errBackend := errors.New("backend died mid-stream")
	gw := &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			body := io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(errBackend))
			ctx.Response = &gateway.Response{
				Status:     http.StatusOK,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(body), -1),
			}
			return nil
		},
	}
	errHandler := &mockErrorHandler{
		handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {},
	}
	routes := gateway.Routes{
		{
			ID:      "r1",
			Timeout: time.Minute,
			Logger:  slog.New(slog.DiscardHandler),
			Predicates: gateway.Predicates{
				predicate.NewMethodPredicate(http.MethodGet),
			},
		},
	}
	server := httptest.NewServer(gatewayhandler.NewGatewayHandler(gw, routes, errHandler))
	defer server.Close()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/test", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	res, err := server.Client().Do(request)
	if err != nil {
		return // the connection was aborted before the headers were flushed: failure observed
	}
	defer res.Body.Close() //nolint:errcheck
	if _, readErr := io.ReadAll(res.Body); readErr == nil {
		t.Fatal("expected the client to observe the truncation, got a complete-looking response")
	}
}

func TestGatewayHandler_ServeHTTP_PropagatesClientCancellation(t *testing.T) {
	clientCtx, cancelClient := context.WithCancel(t.Context())
	cancelClient() // the client is already gone when the handler runs

	var gotErr error
	gw := &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			gotErr = ctx.Err()
			return gateway.ErrHTTP
		},
	}
	errHandler := &mockErrorHandler{
		handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {},
	}
	routes := gateway.Routes{
		{
			ID:      "r1",
			Timeout: time.Minute,
			Predicates: gateway.Predicates{
				predicate.NewMethodPredicate(http.MethodGet),
			},
		},
	}
	gwHandler := gatewayhandler.NewGatewayHandler(gw, routes, errHandler)
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequestWithContext(clientCtx, http.MethodGet, "http://localhost:8080/test", nil)

	gwHandler.ServeHTTP(recorder, request)

	if !errors.Is(gotErr, context.Canceled) {
		t.Errorf("expected gateway context cancelled by client disconnect, actual %v", gotErr)
	}
}
