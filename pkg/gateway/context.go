package gateway

import (
	"context"
	"log/slog"
	"sync"
	"time"
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
	// timeout is the pending route timeout of a streaming route, nil otherwise.
	timeout *time.Timer
}

// NewGatewayContext creates a new gateway context. The route timeout is applied on top
// of the given parent context. The parent must be the inbound request context so that
// client disconnections cancel the proxied backend request instead of letting it run
// until the route timeout expires.
//
// The route timeout is a deadline for the whole exchange, response body included. On a
// streaming route it only runs until Gateway.Do returns: a deadline cannot be lifted
// once set, so the context is cancelled by a timer that is stopped when the response is
// ready to be streamed.
func NewGatewayContext(parent context.Context, route *Route, req *Request) (*Context, context.CancelFunc) {
	gctx := contextPool.Get().(*Context) //nolint:forcetypeassert
	gctx.Request = req
	gctx.Response = nil
	gctx.Route = route
	gctx.Logger = route.Logger

	if route.Streaming {
		return gctx, gctx.armStreamingTimeout(parent, route.Timeout)
	}
	ctx, cancelFunc := context.WithTimeout(parent, route.Timeout)
	gctx.Context = ctx

	return gctx, cancelFunc
}

// routeContextKey is the private key under which a gateway Context exposes its route.
type routeContextKey struct{}

// Value returns the gateway route for the private route key and delegates any other key
// to the embedded context. It lets RouteFromContext find the route even when the gateway
// context has been wrapped by other contexts.
func (c *Context) Value(key any) any {
	if _, ok := key.(routeContextKey); ok {
		if c.Route == nil {
			// untyped nil: a typed nil *Route inside a non-nil any would defeat
			// the nil checks of direct Value callers.
			return nil
		}
		return c.Route
	}
	return c.Context.Value(key)
}

// armStreamingTimeout derives the context of a streaming route. The timeout cancels it
// with context.DeadlineExceeded as the cause, which is the error the http transport
// reports, so a streaming route that times out is indistinguishable from a regular one.
func (c *Context) armStreamingTimeout(parent context.Context, timeout time.Duration) context.CancelFunc {
	ctx, cancel := context.WithCancelCause(parent)
	timer := time.AfterFunc(timeout, func() {
		cancel(context.DeadlineExceeded)
	})
	c.Context = ctx
	c.timeout = timer
	return func() {
		timer.Stop()
		cancel(nil)
	}
}

// disarmTimeout stops the route timeout of a streaming route so that it does not cut
// the response stream. It reports false when the timeout has already fired, in which
// case the context is cancelled and the response must not be streamed. It always
// reports true on regular routes, whose deadline stays in place.
func (c *Context) disarmTimeout() bool {
	if c.timeout == nil {
		return true
	}
	return c.timeout.Stop()
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
	ctx.timeout = nil
	clear(ctx.Attributes)
	contextPool.Put(ctx)
}
