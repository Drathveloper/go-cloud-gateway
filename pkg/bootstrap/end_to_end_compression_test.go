package bootstrap_test

import (
	"bytes"
	"compress/gzip" //nolint:depguard // the backend of the test answers with a gzip body
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	acceptEncodingHeader  = "Accept-Encoding"
	contentEncodingHeader = "Content-Encoding"
	gzipEncoding          = "gzip"
)

// compressionPoolYAML is a complete pool section of the http client.
const compressionPoolYAML = `
    pool:
      timeout: 5s
      keep-alive: 30s
      idle-conn-timeout: 90s
      tls-handshake-timeout: 5s
      response-header-timeout: 10s
      max-idle-conns: 100
      max-idle-conns-per-host: 20
      max-conns-per-host: 50`

func gzipCompress(t *testing.T, content []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(content); err != nil {
		t.Fatalf("failed to compress: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to compress: %v", err)
	}
	return compressed.Bytes()
}

func gzipDecompress(t *testing.T, compressed []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}
	var content bytes.Buffer
	if _, err = content.ReadFrom(reader); err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}
	return content.Bytes()
}

// newNegotiatingBackend returns a backend that answers with the compressed body to the
// requests that accept gzip and with the plain one to the rest, both with their
// Content-Length. The values of the Accept-Encoding header of each request are sent to
// the returned channel.
func newNegotiatingBackend(t *testing.T, plain, compressed []byte) (*httptest.Server, <-chan []string) {
	t.Helper()
	acceptEncodings := make(chan []string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		accepted := request.Header.Values(acceptEncodingHeader)
		acceptEncodings <- accepted
		body := plain
		if slices.Contains(accepted, gzipEncoding) {
			body = compressed
			writer.Header().Set(contentEncodingHeader, gzipEncoding)
		}
		writer.Header().Set("Content-Type", "text/plain")
		writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = writer.Write(body)
	}))
	t.Cleanup(backend.Close)
	return backend, acceptEncodings
}

// newPlainClient returns a client that neither asks for gzip nor decompresses on its own,
// so that the test sees what the gateway sends and the gateway what the test sends.
func newPlainClient(t *testing.T) *http.Client {
	t.Helper()
	transport := &http.Transport{DisableCompression: true}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport}
}

// disable-compression defaults to false whatever the shape of the http client section:
// the gateway then asks the backend for gzip on behalf of the clients that do not ask for
// any encoding and hands them the body decompressed, and leaves alone the exchange of the
// clients that negotiate an encoding themselves.
func TestEndToEnd_Compression(t *testing.T) {
	plain := []byte(strings.Repeat("go-cloud-gateway compresses well. ", 64))
	compressed := gzipCompress(t, plain)
	clients := []struct {
		name           string
		acceptEncoding string
	}{
		{name: "client that sends no Accept-Encoding"},
		{name: "client that sends Accept-Encoding gzip", acceptEncoding: gzipEncoding},
	}
	tests := []struct {
		name       string
		httpClient string
		// gatewayAsksForGzip tells whether the gateway negotiates gzip with the backend
		// for the clients that do not send an Accept-Encoding header.
		gatewayAsksForGzip bool
	}{
		{
			name:               "config without httpclient section",
			httpClient:         "",
			gatewayAsksForGzip: true,
		},
		{
			name:               "config with httpclient section and no pool",
			httpClient:         "\n  httpclient:\n    enable-http2: false",
			gatewayAsksForGzip: true,
		},
		{
			name:               "config with httpclient section and pool",
			httpClient:         "\n  httpclient:" + compressionPoolYAML,
			gatewayAsksForGzip: true,
		},
		{
			name:               "config with disable-compression false and no pool",
			httpClient:         "\n  httpclient:\n    disable-compression: false",
			gatewayAsksForGzip: true,
		},
		{
			name:               "config with disable-compression true and no pool",
			httpClient:         "\n  httpclient:\n    disable-compression: true",
			gatewayAsksForGzip: false,
		},
		{
			name:               "config with disable-compression true and pool",
			httpClient:         "\n  httpclient:\n    disable-compression: true" + compressionPoolYAML,
			gatewayAsksForGzip: false,
		},
	}
	for _, tt := range tests {
		for _, client := range clients {
			t.Run(tt.name+"/"+client.name, func(t *testing.T) {
				backend, acceptEncodings := newNegotiatingBackend(t, plain, compressed)
				gatewayServer := startEndToEndGateway(t, fmt.Sprintf(`
gateway:%s
  routes:
    - id: compression
      uri: %s
`, tt.httpClient, backend.URL))
				request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, gatewayServer.URL+"/test", nil)
				if err != nil {
					t.Fatalf("failed to build request: %v", err)
				}
				if client.acceptEncoding != "" {
					request.Header.Set(acceptEncodingHeader, client.acceptEncoding)
				}

				res := doEndToEndRequest(t, newPlainClient(t), request)

				if res.status != http.StatusOK {
					t.Fatalf("expected status %d actual %d", http.StatusOK, res.status)
				}
				// What the backend is asked for.
				var expectedAccepted []string
				if client.acceptEncoding != "" || tt.gatewayAsksForGzip {
					expectedAccepted = []string{gzipEncoding}
				}
				if accepted := <-acceptEncodings; !slices.Equal(accepted, expectedAccepted) {
					t.Errorf("expected the backend to receive Accept-Encoding %q actual %q", expectedAccepted, accepted)
				}
				// What the client gets: the gzip body as the backend sent it when the
				// client asked for it, the plain one otherwise.
				expectedBody, expectedEncoding := plain, ""
				if client.acceptEncoding != "" {
					expectedBody, expectedEncoding = compressed, gzipEncoding
				}
				if !bytes.Equal(res.body, expectedBody) {
					t.Errorf("expected a body of %d bytes equal to the expected one, actual %d bytes: %q",
						len(expectedBody), len(res.body), res.body)
				}
				if encoding := res.header.Get(contentEncodingHeader); encoding != expectedEncoding {
					t.Errorf("expected Content-Encoding %q actual %q", expectedEncoding, encoding)
				}
				// The gateway may not know the length of a body it decompresses, but a
				// length it declares has to be the one of the body it sends.
				if res.contentLength != -1 && res.contentLength != int64(len(res.body)) {
					t.Errorf("expected Content-Length %d or none actual %d", len(res.body), res.contentLength)
				}
				if client.acceptEncoding != "" {
					if res.contentLength != int64(len(compressed)) {
						t.Errorf("expected Content-Length %d actual %d", len(compressed), res.contentLength)
					}
					if content := gzipDecompress(t, res.body); !bytes.Equal(content, plain) {
						t.Errorf("expected the gzip body to decompress to the plain one, actual %q", content)
					}
				}
			})
		}
	}
}
