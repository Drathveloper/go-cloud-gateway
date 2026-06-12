package gateway

import (
	"context"
	"log/slog"
	"sync"
)

const minAttributesLen = 8

//nolint:gochecknoglobals
var contextPool = sync.Pool{
	New: func() any {
		return &Context{
			Attributes: make(map[string]any, minAttributesLen),
		}
	},
}

// Context represents the gateway context. It holds the relevant information for the gateway to process the request.
type Context struct {
	context.Context //nolint:containedctx

	Request    *Request
	Response   *Response
	Route      *Route
	Logger     *slog.Logger
	Attributes map[string]any
}

// NewGatewayContext creates a new gateway context. The route timeout is applied on top
// of the given parent context. The parent must be the inbound request context so that
// client disconnections cancel the proxied backend request instead of letting it run
// until the route timeout expires.
func NewGatewayContext(parent context.Context, route *Route, req *Request) (*Context, context.CancelFunc) {
	ctx, cancelFunc := context.WithTimeout(parent, route.Timeout)

	gctx := contextPool.Get().(*Context) //nolint:forcetypeassert
	gctx.Request = req
	gctx.Response = nil
	gctx.Route = route
	gctx.Context = ctx
	gctx.Logger = route.Logger

	return gctx, cancelFunc
}

// routeContextKey is the private key under which a gateway Context exposes its route.
type routeContextKey struct{}

// Value returns the gateway route for the private route key and delegates any other key
// to the embedded context. It lets RouteFromContext find the route even when the gateway
// context has been wrapped by other contexts.
func (c *Context) Value(key any) any {
	if _, ok := key.(routeContextKey); ok {
		return c.Route
	}
	return c.Context.Value(key)
}

// RouteFromContext returns the gateway route the given context descends from, or nil when
// there is none. It works through any amount of context wrapping, unlike asserting the
// concrete *Context type, which silently fails on the first context.WithValue.
func RouteFromContext(ctx context.Context) *Route {
	route, _ := ctx.Value(routeContextKey{}).(*Route)
	return route
}

// ReleaseGatewayContext releases the gateway context back to the pool. The handler must call
// it once the request is fully processed, on error paths too. The context must not be used
// after the call.
func ReleaseGatewayContext(ctx *Context) {
	// Drop every reference before pooling: a pooled context would otherwise keep
	// captured bodies and cancelled contexts alive until its next reuse.
	ctx.Request = nil
	ctx.Response = nil
	ctx.Route = nil
	ctx.Logger = nil
	ctx.Context = nil
	clear(ctx.Attributes)
	contextPool.Put(ctx)
}
