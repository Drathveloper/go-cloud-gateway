package gateway

import (
	"context"
	"log/slog"
	"sync"
)

const minAttributesLen = 8

//nolint:gochecknoglobals
var baseContext = context.Background()

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
	Request         *Request
	Response        *Response
	Route           *Route
	Logger          *slog.Logger
	Attributes      map[string]any
	context.Context //nolint:containedctx
}

func clearMap(m map[string]any) {
	for k := range m {
		delete(m, k)
	}
}

// NewGatewayContext creates a new gateway context.
func NewGatewayContext(route *Route, req *Request) (*Context, context.CancelFunc) {
	ctx, cancelFunc := context.WithTimeout(baseContext, route.Timeout)

	gctx := contextPool.Get().(*Context) //nolint:forcetypeassert
	gctx.Request = req
	gctx.Response = nil
	gctx.Route = route
	gctx.Context = ctx
	gctx.Logger = route.Logger

	clearMap(gctx.Attributes)

	return gctx, cancelFunc
}

// ReleaseGatewayContext releases the gateway context. Must be called by the handler after the request is processed.
func ReleaseGatewayContext(ctx *Context) {
	contextPool.Put(ctx)
}
