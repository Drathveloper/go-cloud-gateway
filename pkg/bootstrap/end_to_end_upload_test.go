package bootstrap_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
)

// pacedBody hands out its chunks one per read, waiting the given pause before each one,
// like the body of an upload on a slow link.
type pacedBody struct {
	chunks []string
	pause  time.Duration
}

func (b *pacedBody) Read(output []byte) (int, error) {
	if len(b.chunks) == 0 {
		return 0, io.EOF
	}
	time.Sleep(b.pause)
	read := copy(output, b.chunks[0])
	b.chunks = b.chunks[1:]
	return read, nil
}

// receivedUpload is what a backend got of an upload.
type receivedUpload struct {
	body string
	// contentLength is the length the request declared, -1 when it declared none.
	contentLength int64
	// complete tells whether the body was read up to its end without an error.
	complete bool
}

// newRecordingUploadBackend returns a backend that answers with the request body it
// received. What it received is also sent to the returned channel, once per request.
func newRecordingUploadBackend(t *testing.T) (*httptest.Server, <-chan receivedUpload) {
	t.Helper()
	uploads := make(chan receivedUpload, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		uploads <- receivedUpload{body: string(body), contentLength: request.ContentLength, complete: err == nil}
		if err != nil {
			return
		}
		_, _ = writer.Write(body)
	}))
	t.Cleanup(backend.Close)
	return backend, uploads
}

// startHTTP1Gateway starts the gateway on a plain listener and returns a client of it.
func startHTTP1Gateway(_ *testing.T, gatewayServer *httptest.Server) *http.Client {
	gatewayServer.Start()
	return gatewayServer.Client()
}

// startTLSHTTP2Gateway starts the gateway on a TLS listener that negotiates HTTP/2, the
// way http.Server.ListenAndServeTLS does, and returns a client that trusts its
// certificate.
func startTLSHTTP2Gateway(_ *testing.T, gatewayServer *httptest.Server) *http.Client {
	gatewayServer.EnableHTTP2 = true
	gatewayServer.StartTLS()
	return gatewayServer.Client()
}

// startUnencryptedHTTP2Gateway starts the gateway on a plain listener that also accepts
// unencrypted HTTP/2, which a user enables through the Protocols of the server that
// bootstrap.Initialize returns, and returns a client that only speaks that protocol.
func startUnencryptedHTTP2Gateway(t *testing.T, gatewayServer *httptest.Server) *http.Client {
	t.Helper()
	serverProtocols := new(http.Protocols)
	serverProtocols.SetHTTP1(true)
	serverProtocols.SetUnencryptedHTTP2(true)
	gatewayServer.Config.Protocols = serverProtocols
	gatewayServer.Start()

	clientProtocols := new(http.Protocols)
	clientProtocols.SetUnencryptedHTTP2(true)
	transport := &http.Transport{Protocols: clientProtocols}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport}
}

// The server read timeout is a deadline for the whole request, body included: for the
// whole connection read in HTTP/1 and for each stream in HTTP/2. bootstrap.Initialize
// raises it over the route timeout, so an upload of unknown length is bound by the route
// timeout and not by the configured read timeout, whatever the protocol the client
// speaks to the gateway.
func TestEndToEnd_SlowChunkedUploads(t *testing.T) {
	t.Parallel()
	const (
		readTimeout = 100 * time.Millisecond
		pause       = 60 * time.Millisecond
	)
	chunks := []string{"slow ", "upload ", "of ", "unknown ", "length ", "in ", "seven ", "chunks"}
	upload := strings.Join(chunks, "")
	protocols := []struct {
		start func(t *testing.T, gatewayServer *httptest.Server) *http.Client
		name  string
		major int
	}{
		{name: "HTTP/1.1", start: startHTTP1Gateway, major: 1},
		{name: "HTTP/2 over TLS", start: startTLSHTTP2Gateway, major: 2},
		{name: "unencrypted HTTP/2", start: startUnencryptedHTTP2Gateway, major: 2},
	}
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
			routeTimeout:   "200ms",
			expectedStatus: http.StatusGatewayTimeout,
			expectComplete: false,
		},
	}
	for _, protocol := range protocols {
		for _, tt := range tests {
			t.Run(protocol.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				backend, uploads := newRecordingUploadBackend(t)
				cfg := readYAMLConfig(t, fmt.Sprintf(`
gateway:
  routes:
    - id: upload
      uri: %s
      timeout: %s
`, backend.URL, tt.routeTimeout))
				opts := bootstrap.NewOptionsBuilder(cfg).
					WithServerOptions(bootstrap.ServerOpts{ReadTimeout: readTimeout}).
					Build()
				gatewayServer := newEndToEndGateway(t, opts)
				client := protocol.start(t, gatewayServer)
				request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, gatewayServer.URL+"/upload",
					&pacedBody{chunks: chunks, pause: pause})
				if err != nil {
					t.Fatalf("failed to build request: %v", err)
				}
				// No length is declared: the body is sent chunked in HTTP/1.1 and as data
				// frames without a content-length header in HTTP/2.
				request.ContentLength = -1

				res := doEndToEndRequest(t, client, request)

				if res.protoMajor != protocol.major {
					t.Fatalf("expected the gateway to answer with HTTP/%d actual HTTP/%d", protocol.major, res.protoMajor)
				}
				if res.status != tt.expectedStatus {
					t.Errorf("expected status %d actual %d", tt.expectedStatus, res.status)
				}
				if complete := string(res.body) == upload; complete != tt.expectComplete {
					t.Errorf("expected complete upload answered %t, actual response body %q", tt.expectComplete, res.body)
				}
				received := <-uploads
				if complete := received.complete && received.body == upload; complete != tt.expectComplete {
					t.Errorf("expected complete upload on the backend %t, actual %+v", tt.expectComplete, received)
				}
				if received.contentLength != -1 {
					t.Errorf("expected the backend to receive an upload of unknown length, actual length %d",
						received.contentLength)
				}
			})
		}
	}
}
