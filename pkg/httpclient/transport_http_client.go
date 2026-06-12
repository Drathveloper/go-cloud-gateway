package httpclient

import (
	"net/http"
)

// TransportHTTPClient adapts an http.RoundTripper to the gateway.HTTPClient
// interface, bypassing http.Client entirely.
//
// A gateway must forward 3xx responses to the client untouched, never follow
// them itself, so the only http.Client feature on the gateway path is redirect
// machinery that must stay disabled anyway — and http.Client still pays for it
// per request: a full header-map clone (makeHeadersCopier) and a body rewind
// wrapper (setupRewindBody). RoundTrip never follows redirects, so both
// disappear from the hot path.
type TransportHTTPClient struct {
	// Transport performs the exchange. Exported for configuration inspection,
	// mirroring http.Client.Transport.
	Transport http.RoundTripper
}

// NewTransportHTTPClient creates a new transport-backed http client.
func NewTransportHTTPClient(transport http.RoundTripper) *TransportHTTPClient {
	return &TransportHTTPClient{Transport: transport}
}

// Do performs the request through the transport.
//
// Unlike http.Client.Do, errors come back without a *url.Error wrapper; both
// shapes unwrap to the same root causes, so errors.Is checks against
// context.DeadlineExceeded and context.Canceled behave identically.
//
// There is no client-wide timeout: each request is bounded by its per-route
// context deadline.
func (c *TransportHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.Transport.RoundTrip(req) //nolint:wrapcheck
}
