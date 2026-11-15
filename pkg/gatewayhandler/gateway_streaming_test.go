package gatewayhandler_test

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

// newSlowStreamGateway returns a gateway whose response is a stream of two events
// separated by the given pause.
func newSlowStreamGateway(pause time.Duration) *mockGateway {
	return &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			bodyReader, bodyWriter := io.Pipe()
			go func() {
				_, _ = bodyWriter.Write([]byte("event: one\n\n"))
				time.Sleep(pause)
				_, _ = bodyWriter.Write([]byte("event: two\n\n"))
				_ = bodyWriter.Close()
			}()
			ctx.Response = &gateway.Response{
				Status:     http.StatusOK,
				Headers:    http.Header{"Content-Type": {"text/event-stream"}},
				BodyReader: gateway.NewReplayableBody(bodyReader, -1),
			}
			return nil
		},
	}
}

func newFixedBodyGateway(body string) *mockGateway {
	return &mockGateway{
		doFunc: func(ctx *gateway.Context) error {
			ctx.Response = &gateway.Response{
				Status:     http.StatusOK,
				Headers:    http.Header{},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(strings.NewReader(body)), int64(len(body))),
			}
			return nil
		},
	}
}

func TestGatewayHandler_ServeHTTP_ServerWriteTimeoutOnStreams(t *testing.T) {
	tests := []struct {
		name           string
		streaming      bool
		expectComplete bool
	}{
		{
			name:           "streaming route should outlive the server write timeout",
			streaming:      true,
			expectComplete: true,
		},
		{
			name:           "regular route should be cut by the server write timeout",
			streaming:      false,
			expectComplete: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes := gateway.Routes{
				{
					ID:         "r1",
					Timeout:    time.Minute,
					Streaming:  tt.streaming,
					Logger:     slog.New(slog.DiscardHandler),
					Predicates: gateway.Predicates{predicate.NewMethodPredicate(http.MethodGet)},
				},
			}
			errHandler := &mockErrorHandler{
				handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {},
			}
			gw := newSlowStreamGateway(300 * time.Millisecond)
			server := httptest.NewUnstartedServer(gatewayhandler.NewGatewayHandler(gw, routes, errHandler))
			server.Config.WriteTimeout = 100 * time.Millisecond
			server.Start()
			defer server.Close()

			res, err := server.Client().Do(newTestRequest(t, http.MethodGet, server.URL+"/test", nil))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer res.Body.Close() //nolint:errcheck
			body, err := io.ReadAll(res.Body)

			complete := err == nil && strings.Contains(string(body), "event: two")
			if complete != tt.expectComplete {
				t.Errorf("expected complete stream %t, actual body %q and error %v", tt.expectComplete, body, err)
			}
		})
	}
}

// deadlineRecorder is a response writer that records the write deadlines set on it.
type deadlineRecorder struct {
	*httptest.ResponseRecorder

	deadlines []time.Time
}

func (r *deadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	r.deadlines = append(r.deadlines, deadline)
	return nil
}

func TestGatewayHandler_ServeHTTP_WriteDeadlines(t *testing.T) {
	tests := []struct {
		name      string
		streaming bool
	}{
		{name: "streaming route should bound every write and nothing in between", streaming: true},
		{name: "regular route should leave the server write deadline alone", streaming: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes := gateway.Routes{
				{
					ID:         "r1",
					Timeout:    time.Minute,
					Streaming:  tt.streaming,
					Logger:     slog.New(slog.DiscardHandler),
					Predicates: gateway.Predicates{predicate.NewMethodPredicate(http.MethodGet)},
				},
			}
			errHandler := &mockErrorHandler{
				handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {},
			}
			handler := gatewayhandler.NewGatewayHandler(newFixedBodyGateway("data"), routes, errHandler)
			recorder := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
			start := time.Now()

			handler.ServeHTTP(recorder, newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil))

			if !bytes.Equal(recorder.Body.Bytes(), []byte("data")) {
				t.Errorf("expected body %q actual %q", "data", recorder.Body.Bytes())
			}
			if !tt.streaming {
				if len(recorder.deadlines) != 0 {
					t.Errorf("expected no write deadlines, actual %v", recorder.deadlines)
				}
				return
			}
			// The headers and the only write of the body get a deadline each, cleared
			// once they are on the wire, and the end of the response gets the last one.
			if len(recorder.deadlines) != 5 {
				t.Fatalf("expected 5 write deadlines, actual %v", recorder.deadlines)
			}
			for index, deadline := range recorder.deadlines {
				if index == 1 || index == 3 {
					if !deadline.IsZero() {
						t.Errorf("expected write deadline %d to be cleared, actual %s", index, deadline)
					}
					continue
				}
				if deadline.Before(start.Add(time.Minute)) || deadline.After(time.Now().Add(time.Minute)) {
					t.Errorf("expected write deadline %d one route timeout ahead, actual %s", index, deadline)
				}
			}
		})
	}
}

var errClientWrite = errors.New("write to the client failed")

// brokenWriter is a response writer whose client cannot be written to.
type brokenWriter struct {
	*httptest.ResponseRecorder

	flushErr error
	writeErr error
}

func (w *brokenWriter) Write(output []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.ResponseRecorder.Write(output) //nolint:wrapcheck
}

func (w *brokenWriter) FlushError() error {
	return w.flushErr
}

// A stream that cannot be sent is aborted, so that the server closes the connection
// instead of ending a response the client would take for complete.
func TestGatewayHandler_ServeHTTP_AbortsAStreamThatCannotBeSent(t *testing.T) {
	tests := []struct {
		flushErr error
		writeErr error
		name     string
	}{
		{name: "headers that cannot be flushed should abort the response", flushErr: errClientWrite},
		{name: "body that cannot be written should abort the response", writeErr: errClientWrite},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes := gateway.Routes{
				{
					ID:         "r1",
					Timeout:    time.Minute,
					Streaming:  true,
					Logger:     slog.New(slog.DiscardHandler),
					Predicates: gateway.Predicates{predicate.NewMethodPredicate(http.MethodGet)},
				},
			}
			errHandler := &mockErrorHandler{
				handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {},
			}
			handler := gatewayhandler.NewGatewayHandler(newFixedBodyGateway("data"), routes, errHandler)
			writer := &brokenWriter{
				ResponseRecorder: httptest.NewRecorder(),
				flushErr:         tt.flushErr,
				writeErr:         tt.writeErr,
			}
			defer func() {
				if recovered := recover(); recovered != http.ErrAbortHandler { //nolint:errorlint // sentinel per net/http contract
					t.Errorf("expected http.ErrAbortHandler, actual %v", recovered)
				}
			}()

			handler.ServeHTTP(writer, newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil))

			t.Error("expected the handler to abort the response")
		})
	}
}
