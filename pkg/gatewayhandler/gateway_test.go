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

func TestGatewayHandler_ServeHTTP_FlushesStreamingResponses(t *testing.T) {
	tests := []struct {
		headers     http.Header
		name        string
		bodyLen     int64
		wantFlushed bool
	}{
		{
			name:        "unknown length response is flushed per write",
			headers:     http.Header{},
			bodyLen:     -1,
			wantFlushed: true,
		},
		{
			name:        "server-sent events response is flushed per write",
			headers:     http.Header{"Content-Type": {"text/event-stream; charset=utf-8"}},
			bodyLen:     int64(4),
			wantFlushed: true,
		},
		{
			name:        "known length response is not flushed per write",
			headers:     http.Header{"Content-Type": {"application/json"}},
			bodyLen:     int64(4),
			wantFlushed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := &mockGateway{
				doFunc: func(ctx *gateway.Context) error {
					ctx.Response = &gateway.Response{
						Status:     http.StatusOK,
						Headers:    tt.headers,
						BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBufferString("test")), tt.bodyLen),
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

			gwHandler.ServeHTTP(recorder, newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil))

			if recorder.Flushed != tt.wantFlushed {
				t.Errorf("expected flushed=%v, actual %v", tt.wantFlushed, recorder.Flushed)
			}
		})
	}
}

func TestGatewayHandler_ServeHTTP_StreamsServerSentEvents(t *testing.T) {
	firstEventRead := make(chan struct{})
	bodyReader, bodyWriter := io.Pipe()
	go func() {
		_, _ = bodyWriter.Write([]byte("event: one\n\n"))
		// The client must receive the first event while the stream is still open.
		<-firstEventRead
		_, _ = bodyWriter.Write([]byte("event: two\n\n"))
		_ = bodyWriter.Close()
	}()
	gw := &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			ctx.Response = &gateway.Response{
				Status:     http.StatusOK,
				Headers:    http.Header{"Content-Type": {"text/event-stream"}},
				BodyReader: gateway.NewReplayableBody(bodyReader, -1),
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

	reqCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(reqCtx, http.MethodGet, server.URL+"/test", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	res, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close() //nolint:errcheck

	chunk := make([]byte, 64)
	n, err := res.Body.Read(chunk)
	if err != nil {
		t.Fatalf("expected the first event to arrive while the stream is open, actual error: %v", err)
	}
	if !strings.Contains(string(chunk[:n]), "event: one") {
		t.Fatalf("expected first event, actual %q", chunk[:n])
	}
	close(firstEventRead)
	rest, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("reading rest of stream failed: %v", err)
	}
	if !strings.Contains(string(rest), "event: two") {
		t.Errorf("expected second event, actual %q", rest)
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

func TestGatewayHandler_ServeHTTP_SetsXForwardedHeaders(t *testing.T) {
	var seen http.Header
	gw := &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			seen = ctx.Request.Headers.Clone()
			ctx.Response = &gateway.Response{
				Status:     http.StatusOK,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(nil, 0),
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
	request := newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil)
	request.Host = "gw.example.org"
	request.RemoteAddr = "203.0.113.7:4321"
	request.Header.Set("X-Forwarded-For", "198.51.100.1")

	gwHandler.ServeHTTP(recorder, request)

	for name, want := range map[string]string{
		"X-Forwarded-For":   "198.51.100.1, 203.0.113.7",
		"X-Forwarded-Host":  "gw.example.org",
		"X-Forwarded-Proto": "http",
	} {
		if got := seen.Get(name); got != want {
			t.Errorf("expected backend to see %s=%q, actual %q", name, want, got)
		}
	}
}

func TestGatewayHandler_ServeHTTP_DefaultNotFound(t *testing.T) {
	gw := &mockGateway{
		doFunc: func(_ *gateway.Context) error {
			t.Error("the gateway must not run when no route matches")
			return nil
		},
	}
	errHandler := &mockErrorHandler{
		handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {
			t.Error("the error handler must not run when no route matches")
		},
	}
	routes := gateway.Routes{
		{
			ID: "r1",
			Predicates: gateway.Predicates{
				predicate.NewMethodPredicate(http.MethodPost),
			},
		},
	}
	gwHandler := gatewayhandler.NewGatewayHandler(gw, routes, errHandler)
	recorder := httptest.NewRecorder()

	gwHandler.ServeHTTP(recorder, newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("expected status 404, actual %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "route not found") {
		t.Errorf("expected default not found body, actual %q", recorder.Body.String())
	}
}

func TestGatewayHandler_ServeHTTP_CustomNotFoundHandler(t *testing.T) {
	gw := &mockGateway{
		doFunc: func(_ *gateway.Context) error { return nil },
	}
	errHandler := &mockErrorHandler{
		handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {},
	}
	routes := gateway.Routes{
		{
			ID: "r1",
			Predicates: gateway.Predicates{
				predicate.NewMethodPredicate(http.MethodPost),
			},
		},
	}
	var seenPath string
	notFound := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"route not found"}`))
	})
	gwHandler := gatewayhandler.NewGatewayHandler(gw, routes, errHandler, gatewayhandler.WithNotFoundHandler(notFound))
	recorder := httptest.NewRecorder()

	gwHandler.ServeHTTP(recorder, newTestRequest(t, http.MethodGet, "http://localhost:8080/missing", nil))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("expected status 404, actual %d", recorder.Code)
	}
	if seenPath != "/missing" {
		t.Errorf("expected the custom handler to receive the original request, actual path %q", seenPath)
	}
	if got := recorder.Body.String(); got != `{"error":"route not found"}` {
		t.Errorf("expected custom not found body, actual %q", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("expected custom content type, actual %q", got)
	}
}

func TestGatewayHandler_ServeHTTP_RecoversFilterPanics(t *testing.T) {
	body := &closeCountingBody{Reader: bytes.NewReader([]byte("partial"))}
	gw := &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			ctx.Response = &gateway.Response{
				Status:     http.StatusOK,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(body, int64(len("partial"))),
			}
			panic("filter exploded")
		},
	}
	var handledErr error
	errHandler := &mockErrorHandler{
		handleFunc: func(_ *gateway.Context, err error, w http.ResponseWriter) {
			handledErr = err
			http.Error(w, "", http.StatusInternalServerError)
		},
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
	gwHandler := gatewayhandler.NewGatewayHandler(gw, routes, errHandler)
	recorder := httptest.NewRecorder()

	gwHandler.ServeHTTP(recorder, newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil))

	if !errors.Is(handledErr, gatewayhandler.ErrPanic) {
		t.Errorf("expected the recovered panic as ErrPanic, actual %v", handledErr)
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, actual %d", recorder.Code)
	}
	if body.closes != 1 {
		t.Errorf("expected the response body closed once on the panic path, actual %d", body.closes)
	}
}

func TestGatewayHandler_ServeHTTP_PassesAbortPanicsThrough(t *testing.T) {
	gw := &mockGateway{
		doFunc: func(_ *gateway.Context) error {
			panic(http.ErrAbortHandler)
		},
	}
	errHandler := &mockErrorHandler{
		handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {
			t.Error("the error handler must not run for deliberate aborts")
		},
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

	defer func() {
		if recovered := recover(); recovered != http.ErrAbortHandler { //nolint:errorlint // sentinel per net/http contract
			t.Errorf("expected http.ErrAbortHandler to pass through, actual %v", recovered)
		}
	}()
	gwHandler.ServeHTTP(recorder, newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil))
}

func TestGatewayHandler_ServeHTTP_FilterPanicStillAnswersOverTheWire(t *testing.T) {
	gw := &mockGateway{
		doFunc: func(_ *gateway.Context) error {
			panic("filter exploded")
		},
	}
	errHandler := &mockErrorHandler{
		handleFunc: func(_ *gateway.Context, _ error, w http.ResponseWriter) {
			http.Error(w, "", http.StatusInternalServerError)
		},
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

	res, err := server.Client().Do(newTestRequest(t, http.MethodGet, server.URL+"/test", nil))
	if err != nil {
		t.Fatalf("expected a response instead of a dropped connection, actual error: %v", err)
	}
	defer res.Body.Close() //nolint:errcheck
	if res.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, actual %d", res.StatusCode)
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

// streamObserverRoutes is the single GET route the stream-observer integration tests share.
func streamObserverRoutes() gateway.Routes {
	return gateway.Routes{
		{
			ID:      "r1",
			Timeout: time.Minute,
			Logger:  slog.New(slog.DiscardHandler),
			Predicates: gateway.Predicates{
				predicate.NewMethodPredicate(http.MethodGet),
			},
		},
	}
}

type streamDone struct {
	err   error
	total int64
}

func TestGatewayHandler_ServeHTTP_ObservesServerSentEventsIncrementally(t *testing.T) {
	firstEventRead := make(chan struct{})
	bodyReader, bodyWriter := io.Pipe()
	go func() {
		_, _ = bodyWriter.Write([]byte("event: one\n\n"))
		<-firstEventRead
		_, _ = bodyWriter.Write([]byte("event: two\n\n"))
		_ = bodyWriter.Close()
	}()

	chunkCh := make(chan []byte, 16)
	doneCh := make(chan streamDone, 1)
	gw := &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			// Stand-in for a post-process filter registering an observer on the response.
			ctx.Response = &gateway.Response{
				Status:     http.StatusOK,
				Headers:    http.Header{"Content-Type": {"text/event-stream"}},
				BodyReader: gateway.NewReplayableBody(bodyReader, -1),
			}
			ctx.Response.BodyReader.ObserveStream(
				func(chunk []byte) { chunkCh <- bytes.Clone(chunk) },
				func(total int64, err error) { doneCh <- streamDone{total: total, err: err} },
			)
			return nil
		},
	}
	errHandler := &mockErrorHandler{handleFunc: func(*gateway.Context, error, http.ResponseWriter) {}}
	server := httptest.NewServer(gatewayhandler.NewGatewayHandler(gw, streamObserverRoutes(), errHandler))
	defer server.Close()

	res, err := server.Client().Do(newTestRequest(t, http.MethodGet, server.URL+"/test", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close() //nolint:errcheck

	// The observer sees the first event while the stream is still open: the backend is
	// blocked on firstEventRead and the second event has not been written yet.
	select {
	case got := <-chunkCh:
		if !strings.Contains(string(got), "event: one") {
			t.Fatalf("first observed chunk = %q, want it to contain event: one", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("observer never saw the first event while the stream was open")
	}
	select {
	case <-doneCh:
		t.Fatal("onDone fired before the stream completed")
	default:
	}

	close(firstEventRead)
	select {
	case got := <-chunkCh:
		if !strings.Contains(string(got), "event: two") {
			t.Fatalf("second observed chunk = %q, want it to contain event: two", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("observer never saw the second event")
	}
	if _, err := io.ReadAll(res.Body); err != nil {
		t.Fatalf("draining client stream failed: %v", err)
	}
	select {
	case done := <-doneCh:
		if done.err != nil || done.total != int64(len("event: one\n\nevent: two\n\n")) {
			t.Errorf("onDone total=%d err=%v, want clean completion of the full stream", done.total, done.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onDone never fired after the stream completed")
	}
}

func TestGatewayHandler_ServeHTTP_ObserverNotifiedOnPipelineError(t *testing.T) {
	doneCh := make(chan streamDone, 1)
	gw := &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			ctx.Response = &gateway.Response{
				Status:     http.StatusOK,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(strings.NewReader("data")), -1),
			}
			ctx.Response.BodyReader.ObserveStream(nil, func(total int64, err error) {
				doneCh <- streamDone{total: total, err: err}
			})
			return errors.New("post-filter rejected the response")
		},
	}
	errHandler := &mockErrorHandler{handleFunc: func(*gateway.Context, error, http.ResponseWriter) {}}
	server := httptest.NewServer(gatewayhandler.NewGatewayHandler(gw, streamObserverRoutes(), errHandler))
	defer server.Close()

	res, err := server.Client().Do(newTestRequest(t, http.MethodGet, server.URL+"/test", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = res.Body.Close()

	select {
	case done := <-doneCh:
		if !errors.Is(done.err, gateway.ErrStreamTruncated) {
			t.Errorf("onDone err=%v, want ErrStreamTruncated (response discarded on error)", done.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("observer never notified after the pipeline error closed the response body")
	}
}

func TestGatewayHandler_ServeHTTP_ObserverNotifiedOnMidStreamBackendError(t *testing.T) {
	errBackend := errors.New("backend died mid-stream")
	doneCh := make(chan streamDone, 1)
	gw := &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			body := io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(errBackend))
			ctx.Response = &gateway.Response{
				Status:     http.StatusOK,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(body), -1),
			}
			ctx.Response.BodyReader.ObserveStream(nil, func(total int64, err error) {
				doneCh <- streamDone{total: total, err: err}
			})
			return nil
		},
	}
	errHandler := &mockErrorHandler{handleFunc: func(*gateway.Context, error, http.ResponseWriter) {}}
	server := httptest.NewServer(gatewayhandler.NewGatewayHandler(gw, streamObserverRoutes(), errHandler))
	defer server.Close()

	res, err := server.Client().Do(newTestRequest(t, http.MethodGet, server.URL+"/test", nil))
	if err == nil {
		_, _ = io.ReadAll(res.Body)
		_ = res.Body.Close()
	}
	select {
	case done := <-doneCh:
		if !errors.Is(done.err, errBackend) || done.total != int64(len("partial")) {
			t.Errorf("onDone total=%d err=%v, want the backend error after 7 observed bytes", done.total, done.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("observer never notified of the mid-stream backend failure")
	}
}

func TestGatewayHandler_ServeHTTP_ObservesRequestBody(t *testing.T) {
	t.Run("drained request body is observed and completes cleanly", func(t *testing.T) {
		payload := "request payload"
		chunkCh := make(chan []byte, 16)
		doneCh := make(chan streamDone, 1)
		gw := &mockGateway{
			doFunc: func(ctx *gateway.Context) error {
				ctx.Request.BodyReader.ObserveStream(
					func(chunk []byte) { chunkCh <- bytes.Clone(chunk) },
					func(total int64, err error) { doneCh <- streamDone{total: total, err: err} },
				)
				// Stand-in for the transport reading the body before forwarding it.
				if _, err := io.ReadAll(ctx.Request.BodyReader); err != nil {
					return err //nolint:wrapcheck
				}
				ctx.Response = &gateway.Response{
					Status:     http.StatusOK,
					Headers:    http.Header{},
					BodyReader: gateway.NewReplayableBody(nil, 0),
				}
				return nil
			},
		}
		errHandler := &mockErrorHandler{handleFunc: func(*gateway.Context, error, http.ResponseWriter) {}}
		server := httptest.NewServer(gatewayhandler.NewGatewayHandler(gw, gateway.Routes{
			{
				ID:         "r1",
				Timeout:    time.Minute,
				Logger:     slog.New(slog.DiscardHandler),
				Predicates: gateway.Predicates{predicate.NewMethodPredicate(http.MethodPost)},
			},
		}, errHandler))
		defer server.Close()

		req := newTestRequest(t, http.MethodPost, server.URL+"/test", strings.NewReader(payload))
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		_ = res.Body.Close()

		var done streamDone
		select {
		case done = <-doneCh:
		case <-time.After(2 * time.Second):
			t.Fatal("request-body observer never completed")
		}
		// onChunk always fires before onDone, so every chunk is already buffered by
		// the time onDone arrives: drain what remains without racing the select.
		var observed []byte
		for {
			select {
			case chunk := <-chunkCh:
				observed = append(observed, chunk...)
				continue
			default:
			}
			break
		}
		if done.err != nil || string(observed) != payload {
			t.Errorf("observed %q (total %d) err=%v, want %q cleanly", observed, done.total, done.err, payload)
		}
	})

	t.Run("empty request body completes synchronously at registration", func(t *testing.T) {
		var fired bool
		gw := &mockGateway{
			doFunc: func(ctx *gateway.Context) error {
				// ContentLength 0: ObserveStream must complete before it returns.
				ctx.Request.BodyReader.ObserveStream(nil, func(int64, error) { fired = true })
				if !fired {
					t.Error("onDone for an empty request body did not fire synchronously")
				}
				ctx.Response = &gateway.Response{
					Status:     http.StatusOK,
					Headers:    http.Header{},
					BodyReader: gateway.NewReplayableBody(nil, 0),
				}
				return nil
			},
		}
		errHandler := &mockErrorHandler{handleFunc: func(*gateway.Context, error, http.ResponseWriter) {}}
		server := httptest.NewServer(gatewayhandler.NewGatewayHandler(gw, streamObserverRoutes(), errHandler))
		defer server.Close()

		res, err := server.Client().Do(newTestRequest(t, http.MethodGet, server.URL+"/test", nil))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		_ = res.Body.Close()
	})

	t.Run("untouched request body is closed and observer notified when ServeHTTP returns", func(t *testing.T) {
		doneCh := make(chan streamDone, 1)
		gw := &mockGateway{
			doFunc: func(ctx *gateway.Context) error {
				// Register but never read the body, then fail: only the handler's
				// deferred close can notify the observer.
				ctx.Request.BodyReader.ObserveStream(nil, func(total int64, err error) {
					doneCh <- streamDone{total: total, err: err}
				})
				return errors.New("pre-filter rejected the request")
			},
		}
		errHandler := &mockErrorHandler{handleFunc: func(*gateway.Context, error, http.ResponseWriter) {}}
		server := httptest.NewServer(gatewayhandler.NewGatewayHandler(gw, gateway.Routes{
			{
				ID:         "r1",
				Timeout:    time.Minute,
				Logger:     slog.New(slog.DiscardHandler),
				Predicates: gateway.Predicates{predicate.NewMethodPredicate(http.MethodPost)},
			},
		}, errHandler))
		defer server.Close()

		req := newTestRequest(t, http.MethodPost, server.URL+"/test", strings.NewReader("unread body"))
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		_ = res.Body.Close()

		select {
		case done := <-doneCh:
			if !errors.Is(done.err, gateway.ErrStreamTruncated) {
				t.Errorf("onDone err=%v, want ErrStreamTruncated for an unread request body", done.err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("handler never closed the untouched request body")
		}
	})
}
