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

func ReleaseGatewayContext(ctx *Context) {
	contextPool.Put(ctx)
}
