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
func (g *Gateway) Do(ctx *Context) error {
	if err := ctx.Route.Filters.PreProcessAll(ctx); err != nil {
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, err)
	}
	backendReq := g.buildProxyRequest(ctx)
	backendRes, err := g.httpClient.Do(backendReq) //nolint:bodyclose
	if err != nil {
		return g.handleBackendError(ctx, err)
	}
	ctx.Response = NewGatewayResponse(backendRes)
	if err = ctx.Route.Filters.PostProcessAll(ctx); err != nil {
		// The response never reaches the handler on error: the backend body
		// must be closed here or its connection leaks.
		_ = ctx.Response.BodyReader.Close()
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, err)
	}
	return nil
}

func (g *Gateway) buildProxyRequest(ctx *Context) *http.Request {
	// In place on the shared inbound map: the gateway owns the inbound request for
	// its whole lifetime and the server never re-reads its headers, so a per-request
	// clone would only buy allocations.
	shared.RemoveHopByHopHeaders(ctx.Request.Headers)
	req := &http.Request{
		ContentLength: ctx.Request.BodyReader.Len(),
		Method:        ctx.Request.Method,
		URL:           ctx.Route.GetDestinationURL(ctx.Request.URL),
		Header:        ctx.Request.Headers,
		Body:          ctx.Request.BodyReader,
	}
	if req.ContentLength == 0 {
		// A declared-empty body must be nil: the Transport only retries a request
		// transparently on a connection the backend closed while idle when the
		// body is nil (golang/go#16036), and a nil body also skips the Transport
		// chunked-body probe on every bodyless request.
		req.Body = nil
	}
	// The transport holds the request context in goroutines that outlive this
	// request (queued dials, tracing). It must never see the pooled gateway
	// context, whose fields are reset and reused: it gets the per-request timeout
	// context plus an immutable snapshot of the route instead.
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
