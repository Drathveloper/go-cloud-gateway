package bootstrap_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// newHangingHTTP2Backend returns a TLS backend, HTTP/2 enabled, that never answers and
// records the major version of the protocol of the last request it got.
func newHangingHTTP2Backend(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var protoMajor atomic.Int64
	backend := httptest.NewUnstartedServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		protoMajor.Store(int64(request.ProtoMajor))
		<-request.Context().Done()
	}))
	backend.EnableHTTP2 = true
	backend.StartTLS()
	t.Cleanup(backend.Close)
	return backend, &protoMajor
}

// The context of a streaming route is cancelled with the timeout as its cause, but its
// Err is context.Canceled, which is what the HTTP/2 transport of net/http reports. A
// streaming route to an HTTP/2 backend was answered 499 Client Closed Request when it
// timed out, and the circuit breaker did not count the timeout as a failure.
func TestEndToEnd_RouteTimeoutToAnHTTP2Backend(t *testing.T) {
	const breaker = `
      circuit-breaker:
        enabled: true
        interval: 1m
        min-requests-threshold: 1
        failure-rate-threshold: 1
        wait-duration-in-open-state: 1m
        num-allowed-half-open-calls: 1`
	tests := []struct {
		name           string
		routeExtra     string
		expectedStatus []int
		streaming      bool
	}{
		{name: "regular route", expectedStatus: []int{http.StatusGatewayTimeout}},
		{name: "streaming route", streaming: true, expectedStatus: []int{http.StatusGatewayTimeout}},
		{
			name:           "regular route with a circuit breaker that opens at the first failure",
			routeExtra:     breaker,
			expectedStatus: []int{http.StatusGatewayTimeout, http.StatusServiceUnavailable},
		},
		{
			name:           "streaming route with a circuit breaker that opens at the first failure",
			routeExtra:     breaker,
			streaming:      true,
			expectedStatus: []int{http.StatusGatewayTimeout, http.StatusServiceUnavailable},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, protoMajor := newHangingHTTP2Backend(t)
			gatewayServer := startEndToEndGateway(t, fmt.Sprintf(`
gateway:
  httpclient:
    insecure-tls-verify: true
    enable-http2: true
  routes:
    - id: hanging
      uri: %s
      timeout: 100ms
      streaming: %t%s
`, backend.URL, tt.streaming, tt.routeExtra))

			for i, expected := range tt.expectedStatus {
				request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, gatewayServer.URL+"/test", nil)
				if err != nil {
					t.Fatalf("failed to build request: %v", err)
				}

				res := doEndToEndRequest(t, gatewayServer.Client(), request)

				if res.status != expected {
					t.Errorf("request %d: expected status %d actual %d", i, expected, res.status)
				}
			}
			if got := protoMajor.Load(); got != 2 {
				t.Errorf("expected the backend to be called over HTTP/2, actual HTTP/%d", got)
			}
		})
	}
}
