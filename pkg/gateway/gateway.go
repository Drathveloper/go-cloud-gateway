package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
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
	httpClient HTTPClient
}

// NewGateway creates a new gateway.
func NewGateway(client HTTPClient) *Gateway {
	return &Gateway{
		httpClient: client,
	}
}

// Do process the gateway request. It will call all pre-process filters, the backend and the post-process filters.
// It will return an error if the gateway request failed.
// If the gateway request and filters are successful, it will return nil.
//
// On a streaming route the route timeout stops running once Do returns successfully,
// so the response body can be streamed for as long as it lasts.
//
// A context.Canceled that the timeout of a streaming route caused, from a filter or from
// the http client, is reported as the context.DeadlineExceeded of a regular route: see
// shared.AsRouteTimeout.
func (g *Gateway) Do(ctx *Context) error {
	if err := ctx.Route.Filters.PreProcessAll(ctx); err != nil {
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, shared.AsRouteTimeout(ctx, err))
	}
	backendReq := g.buildProxyRequest(ctx)
	backendRes, err := g.httpClient.Do(backendReq) //nolint:bodyclose
	if err != nil {
		return g.handleBackendError(ctx, shared.AsRouteTimeout(ctx, err))
	}
	ctx.Response = NewGatewayResponse(backendRes)
	if err = ctx.Route.Filters.PostProcessAll(ctx); err != nil {
		_ = ctx.Response.BodyReader.Close()
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, shared.AsRouteTimeout(ctx, err))
	}
	if !ctx.disarmTimeout() {
		// The timeout of a streaming route fired while the post-process filters
		// ran: the context is already cancelled, so the body cannot be streamed.
		_ = ctx.Response.BodyReader.Close()
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, context.DeadlineExceeded)
	}
	return nil
}

func (g *Gateway) buildProxyRequest(ctx *Context) *http.Request {
	shared.RemoveHopByHopHeaders(ctx.Request.Headers)
	req := &http.Request{
		ContentLength: ctx.Request.BodyReader.Len(),
		Method:        ctx.Request.Method,
		URL:           ctx.Route.GetDestinationURL(ctx.Request.URL),
		Header:        ctx.Request.Headers,
		Body:          ctx.Request.BodyReader,
	}
	if req.ContentLength == 0 {
		req.Body = nil
	}
	return req.WithContext(context.WithValue(ctx.Context, routeContextKey{}, ctx.Route))
}

func (g *Gateway) handleBackendError(ctx *Context, err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, context.DeadlineExceeded)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, context.Canceled)
	case errors.Is(err, circuitbreaker.ErrOpenState) || errors.Is(err, circuitbreaker.ErrHalfOpenRequestExceeded):
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, fmt.Errorf("%w: %s", ErrCircuitBreaker, err.Error()))
	default:
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, fmt.Errorf("%w: %s", ErrHTTP, err.Error()))
	}
}
