package httpclient

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// ErrInternalServer is returned when http 5xx error occurs.
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
	switch ctx := req.Context().(type) {
	case *gateway.Context:
		if ctx.Route.CircuitBreaker == nil {
			return c.client.Do(req) //nolint:wrapcheck
		}
		return c.doWithCircuitBreaker(ctx.Route.CircuitBreaker, req)
	default:
		return c.client.Do(req) //nolint:wrapcheck
	}
}

func (c *CircuitBreakerHTTPClient) doWithCircuitBreaker(
	circuitBreaker gateway.CircuitBreaker[*http.Response], req *http.Request) (*http.Response, error) {
	result, err := circuitBreaker.Execute(func() (*http.Response, error) {
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("wrapped circuit breaker request failed: %w", err)
		}
		if resp.StatusCode >= internalServerErrorStatusCode {
			return nil, ErrInternalServer
		}
		return resp, nil
	})
	if err != nil {
		return nil, err //nolint:wrapcheck
	}
	return result, nil
}
