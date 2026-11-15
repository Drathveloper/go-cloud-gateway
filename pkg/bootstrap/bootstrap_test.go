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
	return startTestGatewayWithOptions(t, builder.Build())
}

// startTestGatewayWithOptions serves the gateway built from the given options on a test
// server that keeps the timeouts of the initialized server.
func startTestGatewayWithOptions(t *testing.T, opts *bootstrap.Options) *httptest.Server {
	t.Helper()
	server, err := bootstrap.Initialize(opts)
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

// slowReader hands out its chunks one per read, waiting the given pause before each one,
// like a client on a slow link does.
type slowReader struct {
	chunks []string
	pause  time.Duration
}

func (r *slowReader) Read(output []byte) (int, error) {
	if len(r.chunks) == 0 {
		return 0, io.EOF
	}
	time.Sleep(r.pause)
	read := copy(output, r.chunks[0])
	r.chunks = r.chunks[1:]
	return read, nil
}

// newUploadBackend returns a backend that answers with the request body it received. The
// body is also sent to the returned channel, complete or not, once per request.
func newUploadBackend(t *testing.T) (*httptest.Server, <-chan string) {
	t.Helper()
	received := make(chan string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		received <- string(body)
		if err != nil {
			return
		}
		_, _ = writer.Write(body)
	}))
	t.Cleanup(backend.Close)
	return backend, received
}

// postTestUpload sends the chunks as the body of a request of known length, one chunk
// per pause, and returns the status code and the body of the response.
func postTestUpload(
	t *testing.T,
	client *http.Client,
	url string,
	chunks []string,
	pause time.Duration,
) (int, string) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url,
		&slowReader{chunks: chunks, pause: pause})
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	request.ContentLength = int64(len(strings.Join(chunks, "")))
	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return res.StatusCode, string(body)
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

func TestInitialize_ServerReadTimeout(t *testing.T) {
	tests := []struct {
		name         string
		routeTimeout string
		readTimeout  time.Duration
		expected     time.Duration
	}{
		{
			name:         "read timeout shorter than the route timeout should be raised over it",
			routeTimeout: "30s",
			readTimeout:  10 * time.Second,
			expected:     35 * time.Second,
		},
		{
			name:         "read timeout equal to the route timeout should be raised over it",
			routeTimeout: "10s",
			readTimeout:  10 * time.Second,
			expected:     15 * time.Second,
		},
		{
			name:         "read timeout that covers the route timeout should be kept",
			routeTimeout: "10s",
			readTimeout:  time.Minute,
			expected:     time.Minute,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := readTestConfig(t,
				`{"gateway":{"routes":[{"id":"r1","uri":"http://localhost:8001","timeout":"`+tt.routeTimeout+`"}]}}`)
			builder := bootstrap.NewOptionsBuilder(cfg).
				WithServerOptions(bootstrap.ServerOpts{ReadTimeout: tt.readTimeout})

			server, err := bootstrap.Initialize(builder.Build())
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}

			if server.ReadTimeout != tt.expected {
				t.Errorf("expected read timeout %s actual %s", tt.expected, server.ReadTimeout)
			}
		})
	}
}

func TestInitialize_DisabledServerReadTimeoutIsKept(t *testing.T) {
	cfg := readTestConfig(t, `{"gateway":{"routes":[{"id":"r1","uri":"http://localhost:8001","timeout":"30s"}]}}`)
	opts := bootstrap.NewOptionsBuilder(cfg).Build()
	opts.ServerOptions.ReadTimeout = 0

	server, err := bootstrap.Initialize(opts)
	if err != nil {
		t.Fatalf("expected nil err actual %v", err)
	}

	if server.ReadTimeout != 0 {
		t.Errorf("expected read timeout disabled, actual %s", server.ReadTimeout)
	}
}

// net/http uses the read timeout for a header or idle timeout left at zero: raising the
// read timeout must not raise them as well.
func TestInitialize_RaisedServerReadTimeoutKeepsHeaderAndIdleTimeouts(t *testing.T) {
	tests := []struct {
		name               string
		serverOpts         bootstrap.ServerOpts
		expectedReadHeader time.Duration
		expectedIdle       time.Duration
	}{
		{
			name:               "unset header and idle timeouts should keep the configured read timeout",
			serverOpts:         bootstrap.ServerOpts{ReadTimeout: 10 * time.Second},
			expectedReadHeader: 10 * time.Second,
			expectedIdle:       10 * time.Second,
		},
		{
			name: "configured header and idle timeouts should be kept",
			serverOpts: bootstrap.ServerOpts{
				ReadTimeout:       10 * time.Second,
				ReadHeaderTimeout: 2 * time.Second,
				IdleTimeout:       time.Minute,
			},
			expectedReadHeader: 2 * time.Second,
			expectedIdle:       time.Minute,
		},
		{
			name:               "unset header and idle timeouts should stay unset when the read timeout is not raised",
			serverOpts:         bootstrap.ServerOpts{ReadTimeout: time.Minute},
			expectedReadHeader: 0,
			expectedIdle:       0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := readTestConfig(t,
				`{"gateway":{"routes":[{"id":"r1","uri":"http://localhost:8001","timeout":"30s"}]}}`)
			opts := bootstrap.NewOptionsBuilder(cfg).Build()
			opts.ServerOptions = tt.serverOpts

			server, err := bootstrap.Initialize(opts)
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}

			if server.ReadHeaderTimeout != tt.expectedReadHeader {
				t.Errorf("expected read header timeout %s actual %s", tt.expectedReadHeader, server.ReadHeaderTimeout)
			}
			if server.IdleTimeout != tt.expectedIdle {
				t.Errorf("expected idle timeout %s actual %s", tt.expectedIdle, server.IdleTimeout)
			}
		})
	}
}

// The server read timeout is a deadline for the whole request, body included, and starts
// counting before the route timeout does: when it is not the longer of the two, an upload
// is cut by it however long the route timeout is.
func TestInitialize_RouteTimeoutOnSlowUploads(t *testing.T) {
	const pause = 150 * time.Millisecond
	chunks := []string{"slow ", "upload ", "in ", "five ", "chunks"}
	upload := strings.Join(chunks, "")
	tests := []struct {
		name           string
		routeTimeout   string
		expectedStatus int
		expectComplete bool
	}{
		{
			name:           "upload slower than the read timeout should be delivered within the route timeout",
			routeTimeout:   "5s",
			expectedStatus: http.StatusOK,
			expectComplete: true,
		},
		{
			name:           "upload slower than the route timeout should be cut",
			routeTimeout:   "300ms",
			expectedStatus: http.StatusGatewayTimeout,
			expectComplete: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, received := newUploadBackend(t)
			cfgJSON := fmt.Sprintf(`{"gateway":{"routes":[{"id":"r1","uri":%q,"timeout":%q}]}}`,
				backend.URL, tt.routeTimeout)
			gatewayServer := startTestGateway(t, cfgJSON, bootstrap.ServerOpts{ReadTimeout: 200 * time.Millisecond})

			status, body := postTestUpload(t, gatewayServer.Client(), gatewayServer.URL+"/test", chunks, pause)

			if status != tt.expectedStatus {
				t.Errorf("expected status %d actual %d", tt.expectedStatus, status)
			}
			if complete := body == upload; complete != tt.expectComplete {
				t.Errorf("expected complete upload answered %t, actual response body %q", tt.expectComplete, body)
			}
			if complete := <-received == upload; complete != tt.expectComplete {
				t.Errorf("expected complete upload on the backend %t actual %t", tt.expectComplete, complete)
			}
		})
	}
}
