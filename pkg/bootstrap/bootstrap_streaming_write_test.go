package bootstrap_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
)

const streamEvents = 4

// newEventBackend returns a backend that sends its headers at once and then, after
// headerWait, a stream of events separated by gap. It ends the stream endWait after the
// last event.
func newEventBackend(tb testing.TB, headerWait, gap, endWait time.Duration) *httptest.Server {
	tb.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		controller := http.NewResponseController(writer)
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)
		_ = controller.Flush()
		time.Sleep(headerWait)
		for event := range streamEvents {
			if event > 0 {
				time.Sleep(gap)
			}
			_, _ = fmt.Fprintf(writer, "data: %d\n\n", event)
			_ = controller.Flush()
		}
		time.Sleep(endWait)
	}))
	tb.Cleanup(backend.Close)
	return backend
}

// newStreamingGateway starts a gateway with one streaming route to the backend, with the
// given route timeout, over HTTP/1.1 or over HTTP/2.
func newStreamingGateway(tb testing.TB, backendURL string, timeout time.Duration, http2 bool) *httptest.Server {
	tb.Helper()
	yml := fmt.Sprintf(`
gateway:
  routes:
    - id: events
      uri: %s
      timeout: %s
      streaming: true
      predicates:
        - name: Path
          args:
            patterns:
              - /**
`, backendURL, timeout)
	cfg, err := config.NewReaderYAML(validator.New()).Read([]byte(yml))
	if err != nil {
		tb.Fatalf("read config failed: %v", err)
	}
	server, err := bootstrap.Initialize(bootstrap.NewOptionsBuilder(cfg).Build())
	if err != nil {
		tb.Fatalf("initialize failed: %v", err)
	}
	gatewayServer := httptest.NewUnstartedServer(server.Handler)
	gatewayServer.Config.ReadTimeout = server.ReadTimeout
	gatewayServer.Config.WriteTimeout = server.WriteTimeout
	if http2 {
		gatewayServer.EnableHTTP2 = true
		gatewayServer.StartTLS()
	} else {
		gatewayServer.Start()
	}
	tb.Cleanup(gatewayServer.Close)
	return gatewayServer
}

// getEvents asks the gateway for the stream of events.
func getEvents(tb testing.TB, gatewayServer *httptest.Server) (*http.Response, error) {
	tb.Helper()
	request, err := http.NewRequestWithContext(tb.Context(), http.MethodGet, gatewayServer.URL+"/events", nil)
	if err != nil {
		tb.Fatalf("build request failed: %v", err)
	}
	return gatewayServer.Client().Do(request) //nolint:wrapcheck
}

// The route timeout of a streaming route bounds every write, not the time between two
// of them nor the time the stream takes to end: a stream that stays quiet for longer
// than the timeout must reach the client whole and end cleanly.
func TestInitialize_StreamingRouteQuietStreams(t *testing.T) {
	t.Parallel()
	const timeout = 100 * time.Millisecond
	tests := []struct {
		name    string
		gap     time.Duration
		endWait time.Duration
	}{
		{name: "events further apart than the route timeout", gap: 250 * time.Millisecond},
		{name: "stream that ends long after its last event", gap: 10 * time.Millisecond, endWait: 250 * time.Millisecond},
		{name: "events closer than the route timeout", gap: 10 * time.Millisecond},
	}
	for _, protocol := range []struct {
		name  string
		proto string
		http2 bool
	}{
		{name: "http/1.1", proto: "HTTP/1.1"},
		{name: "http/2", proto: "HTTP/2.0", http2: true},
	} {
		for _, tt := range tests {
			t.Run(protocol.name+" "+tt.name, func(t *testing.T) {
				t.Parallel()
				backend := newEventBackend(t, 0, tt.gap, tt.endWait)
				gatewayServer := newStreamingGateway(t, backend.URL, timeout, protocol.http2)

				res, err := getEvents(t, gatewayServer)
				if err != nil {
					t.Fatalf("request failed: %v", err)
				}
				defer res.Body.Close() //nolint:errcheck
				body, err := io.ReadAll(res.Body)

				if res.Proto != protocol.proto {
					t.Errorf("expected protocol %s actual %s", protocol.proto, res.Proto)
				}
				if err != nil {
					t.Errorf("expected the stream to end cleanly, actual error %v", err)
				}
				if events := strings.Count(string(body), "data:"); events != streamEvents {
					t.Errorf("expected %d events actual %d", streamEvents, events)
				}
			})
		}
	}
}

// The headers of a stream reach the client before its first event: a client learns that
// the stream is open from them.
func TestInitialize_StreamingRouteSendsHeadersBeforeTheFirstEvent(t *testing.T) {
	t.Parallel()
	const headerWait = 400 * time.Millisecond
	for _, protocol := range []struct {
		name  string
		http2 bool
	}{
		{name: "http/1.1"},
		{name: "http/2", http2: true},
	} {
		t.Run(protocol.name, func(t *testing.T) {
			t.Parallel()
			backend := newEventBackend(t, headerWait, 0, 0)
			gatewayServer := newStreamingGateway(t, backend.URL, 5*time.Second, protocol.http2)
			start := time.Now()

			res, err := getEvents(t, gatewayServer)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer res.Body.Close() //nolint:errcheck
			headersAfter := time.Since(start)
			body, err := io.ReadAll(res.Body)

			if headersAfter >= headerWait/2 {
				t.Errorf("expected the headers before the first event, which comes after %s, actual after %s",
					headerWait, headersAfter)
			}
			if err != nil {
				t.Errorf("expected the stream to end cleanly, actual error %v", err)
			}
			if events := strings.Count(string(body), "data:"); events != streamEvents {
				t.Errorf("expected %d events actual %d", streamEvents, events)
			}
		})
	}
}

// A deadline per write still drops a client that stops reading: the write that waits
// for it fails once the route timeout has passed, and the backend is released.
func TestInitialize_StreamingRouteDropsAClientThatStopsReading(t *testing.T) {
	t.Parallel()
	for _, protocol := range []struct {
		name  string
		http2 bool
	}{
		{name: "http/1.1"},
		{name: "http/2", http2: true},
	} {
		t.Run(protocol.name, func(t *testing.T) {
			t.Parallel()
			released := make(chan struct{})
			backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				defer close(released)
				chunk := make([]byte, 64*1024)
				// It only stops when the gateway hangs up.
				for {
					if _, err := writer.Write(chunk); err != nil {
						return
					}
				}
			}))
			t.Cleanup(backend.Close)
			gatewayServer := newStreamingGateway(t, backend.URL, 100*time.Millisecond, protocol.http2)

			res, err := getEvents(t, gatewayServer)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			// The body is left unread until the backend has been released.
			defer res.Body.Close() //nolint:errcheck

			select {
			case <-released:
			case <-time.After(3 * time.Second):
				t.Fatal("expected the gateway to drop the client that stopped reading and release the backend")
			}
		})
	}
}
