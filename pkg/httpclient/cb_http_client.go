package httpclient

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// ErrInternalServer marks backend 5xx responses as failures for the circuit breaker
// accounting. It is not returned to callers: the 5xx response itself is.
var ErrInternalServer = errors.New("internal server error")

const internalServerErrorStatusCode = 500

// CircuitBreakerHTTPClient is a circuit breaker http client.
type CircuitBreakerHTTPClient struct {
	client gateway.HTTPClient
}

// NewCircuitBreakerHTTPClient creates a new circuit breaker http client.
//
// The circuit breaker will be applied to the matching route.
func NewCircuitBreakerHTTPClient(client gateway.HTTPClient) *CircuitBreakerHTTPClient {
	return &CircuitBreakerHTTPClient{
		client: client,
	}
}

// Do execute the request applying the matching route circuit breaker.
func (c *CircuitBreakerHTTPClient) Do(req *http.Request) (*http.Response, error) {
	route := gateway.RouteFromContext(req.Context())
	if route == nil || route.CircuitBreaker == nil {
		return c.client.Do(req) //nolint:wrapcheck
	}
	return c.doWithCircuitBreaker(route.CircuitBreaker, req)
}

func (c *CircuitBreakerHTTPClient) doWithCircuitBreaker(
	circuitBreaker gateway.CircuitBreaker[*http.Response], req *http.Request) (*http.Response, error) {
	result, err := circuitBreaker.Execute(func() (*http.Response, error) {
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("wrapped circuit breaker request failed: %w", err)
		}
		if resp.StatusCode >= internalServerErrorStatusCode {
			// The error makes the breaker count a failure, but the response still
			// travels back so the client receives the real backend status and body
			// instead of a generic gateway error.
			return resp, ErrInternalServer
		}
		return resp, nil
	})
	if result != nil && err != nil {
		// A response alongside an error is the 5xx accounting case: the breaker
		// already counted the failure, the caller gets the response.
		return result, nil //nolint:nilerr // the error only feeds the breaker accounting
	}
	if err != nil {
		return nil, err //nolint:wrapcheck
	}
	return result, nil
}
