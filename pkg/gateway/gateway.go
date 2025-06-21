package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/pkg/circuitbreaker"
)

// ErrHTTP is the error returned when the gateway http request to backend failed.
var ErrHTTP = errors.New("gateway http request to backend failed")

// ErrCircuitBreaker is the error returned when the circuit breaker failed.
var ErrCircuitBreaker = errors.New("circuit breaker failed")

// HTTPClient is the interface for the http client.
type HTTPClient interface {
	// Do perform the http request. It returns the http response and an error if the request failed.
	Do(r *http.Request) (*http.Response, error)
}

const gatewayErrMsg = "gateway request for route %s failed: %w"

// Gateway is the gateway struct. It holds the gateway configuration and the http client.
type Gateway struct {
	httpClient    HTTPClient
	globalFilters Filters
}

// NewGateway creates a new gateway.
func NewGateway(globalFilters Filters, client HTTPClient) *Gateway {
	return &Gateway{
		globalFilters: globalFilters,
		httpClient:    client,
	}
}

// Do process the gateway request. It will call the global filters, the route filters, and the backend.
// It will return an error if the gateway request failed.
// If the gateway request and filters are successful, it will return nil.
func (g *Gateway) Do(ctx *Context) error {
	allFilters := ctx.Route.CombineGlobalFilters(g.globalFilters...)
	if err := allFilters.PreProcessAll(ctx); err != nil {
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, err)
	}
	backendReq := g.buildProxyRequest(ctx)
	backendRes, err := g.httpClient.Do(backendReq) //nolint:bodyclose
	if err != nil {
		return g.handleBackendError(ctx, err)
	}
	ctx.Response = NewGatewayResponse(backendRes)
	if err = allFilters.PostProcessAll(ctx); err != nil {
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, err)
	}
	return nil
}

func (g *Gateway) handleBackendError(ctx *Context, err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, context.DeadlineExceeded)
	case errors.Is(err, circuitbreaker.ErrOpenState) || errors.Is(err, circuitbreaker.ErrHalfOpenRequestExceeded):
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, fmt.Errorf("%w: %s", ErrCircuitBreaker, err.Error()))
	default:
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, fmt.Errorf("%w: %s", ErrHTTP, err.Error()))
	}
}

func (g *Gateway) buildProxyRequest(ctx *Context) *http.Request {
	req := &http.Request{
		ContentLength: ctx.Request.BodyReader.Len(),
		Method:        ctx.Request.Method,
		URL:           ctx.Route.GetDestinationURL(ctx.Request.URL),
		Header:        ctx.Request.Headers,
		Body:          ctx.Request.BodyReader,
	}
	return req.WithContext(ctx)
}
