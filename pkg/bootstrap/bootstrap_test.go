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

func readTestConfig(t *testing.T, cfgJSON string) *config.Config {
	t.Helper()
	cfg, err := config.NewReaderJSON(validator.New()).Read([]byte(cfgJSON))
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	return cfg
}

// startTestGateway serves the gateway built from the given config on a test server that
// keeps the timeouts of the initialized server.
func startTestGateway(t *testing.T, cfgJSON string, serverOpts bootstrap.ServerOpts) *httptest.Server {
	t.Helper()
	builder := bootstrap.NewOptionsBuilder(readTestConfig(t, cfgJSON)).WithServerOptions(serverOpts)
	server, err := bootstrap.Initialize(builder.Build())
	if err != nil {
		t.Fatalf("failed to initialize gateway: %v", err)
	}
	testServer := httptest.NewUnstartedServer(server.Handler)
	testServer.Config = server
	testServer.Start()
	t.Cleanup(testServer.Close)
	return testServer
}

// newHangingBackend returns a backend that never answers.
func newHangingBackend(t *testing.T) *httptest.Server {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	t.Cleanup(backend.Close)
	return backend
}

// newEventStreamBackend returns a backend that streams the given number of server-sent
// events, one per interval.
func newEventStreamBackend(t *testing.T, events int, interval time.Duration) *httptest.Server {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		controller := http.NewResponseController(writer)
		for i := range events {
			if _, err := fmt.Fprintf(writer, "data: %d\n\n", i); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
			time.Sleep(interval)
		}
	}))
	t.Cleanup(backend.Close)
	return backend
}

// getTestURL returns the status code of the response, its body and the error of reading
// the body, if any.
func getTestURL(t *testing.T, client *http.Client, url string) (int, []byte, error) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(res.Body)
	return res.StatusCode, body, err //nolint:wrapcheck
}

func TestInitialize_ServerWriteTimeout(t *testing.T) {
	tests := []struct {
		name         string
		routeTimeout string
		writeTimeout time.Duration
		expected     time.Duration
	}{
		{
			name:         "write timeout shorter than the route timeout should be raised over it",
			routeTimeout: "30s",
			writeTimeout: 10 * time.Second,
			expected:     35 * time.Second,
		},
		{
			name:         "write timeout equal to the route timeout should be raised over it",
			routeTimeout: "10s",
			writeTimeout: 10 * time.Second,
			expected:     15 * time.Second,
		},
		{
			name:         "write timeout that covers the route timeout should be kept",
			routeTimeout: "10s",
			writeTimeout: time.Minute,
			expected:     time.Minute,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := readTestConfig(t,
				`{"gateway":{"routes":[{"id":"r1","uri":"http://localhost:8001","timeout":"`+tt.routeTimeout+`"}]}}`)
			builder := bootstrap.NewOptionsBuilder(cfg).
				WithServerOptions(bootstrap.ServerOpts{WriteTimeout: tt.writeTimeout})

			server, err := bootstrap.Initialize(builder.Build())
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}

			if server.WriteTimeout != tt.expected {
				t.Errorf("expected write timeout %s actual %s", tt.expected, server.WriteTimeout)
			}
		})
	}
}

func TestInitialize_DisabledServerWriteTimeoutIsKept(t *testing.T) {
	cfg := readTestConfig(t, `{"gateway":{"routes":[{"id":"r1","uri":"http://localhost:8001","timeout":"30s"}]}}`)
	opts := bootstrap.NewOptionsBuilder(cfg).Build()
	opts.ServerOptions.WriteTimeout = 0

	server, err := bootstrap.Initialize(opts)
	if err != nil {
		t.Fatalf("expected nil err actual %v", err)
	}

	if server.WriteTimeout != 0 {
		t.Errorf("expected write timeout disabled, actual %s", server.WriteTimeout)
	}
}

// The server write timeout starts counting before the route timeout does: when it is not
// the longer of the two, the connection is closed before the 504 can be written.
func TestInitialize_RouteTimeoutAnswersGatewayTimeout(t *testing.T) {
	tests := []struct {
		name      string
		streaming bool
	}{
		{name: "regular route should answer 504 when the backend does not reply in time", streaming: false},
		{name: "streaming route should answer 504 when the backend does not reply in time", streaming: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newHangingBackend(t)
			cfgJSON := fmt.Sprintf(
				`{"gateway":{"routes":[{"id":"r1","uri":%q,"timeout":"200ms","streaming":%t}]}}`,
				backend.URL, tt.streaming)
			gatewayServer := startTestGateway(t, cfgJSON, bootstrap.ServerOpts{WriteTimeout: 100 * time.Millisecond})

			status, _, err := getTestURL(t, gatewayServer.Client(), gatewayServer.URL+"/test")
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}

			if status != http.StatusGatewayTimeout {
				t.Errorf("expected status %d actual %d", http.StatusGatewayTimeout, status)
			}
		})
	}
}

func TestInitialize_RouteTimeoutOnStreams(t *testing.T) {
	const events = 5
	tests := []struct {
		name           string
		streaming      bool
		expectComplete bool
	}{
		{
			name:           "streaming route should deliver a stream longer than its timeout",
			streaming:      true,
			expectComplete: true,
		},
		{
			name:           "regular route should cut a stream longer than its timeout",
			streaming:      false,
			expectComplete: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newEventStreamBackend(t, events, 100*time.Millisecond)
			cfgJSON := fmt.Sprintf(
				`{"gateway":{"routes":[{"id":"r1","uri":%q,"timeout":"200ms","streaming":%t}]}}`,
				backend.URL, tt.streaming)
			gatewayServer := startTestGateway(t, cfgJSON, bootstrap.ServerOpts{})

			status, body, err := getTestURL(t, gatewayServer.Client(), gatewayServer.URL+"/test")

			if status != http.StatusOK {
				t.Fatalf("expected status %d actual %d", http.StatusOK, status)
			}
			complete := err == nil && strings.Count(string(body), "data:") == events
			if complete != tt.expectComplete {
				t.Errorf("expected complete stream %t, actual body %q and error %v", tt.expectComplete, body, err)
			}
		})
	}
}
