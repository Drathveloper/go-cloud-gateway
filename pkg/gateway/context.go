package gateway

import (
	"context"
	"log/slog"
	"sync"
)

var baseContext = context.Background()

var contextPool = sync.Pool{
	New: func() any {
		return &Context{
			Attributes: make(map[string]any, 8),
		}
	},
}

type Context struct {
	Request    *Request
	Response   *Response
	Route      *Route
	Attributes map[string]any
	Logger     *slog.Logger
	context.Context
}

func clearMap(m map[string]any) {
	for k := range m {
		delete(m, k)
	}
}

func NewGatewayContext(route *Route, req *Request) (*Context, context.CancelFunc) {
	ctx, cancelFunc := context.WithTimeout(baseContext, route.Timeout)

	gctx := contextPool.Get().(*Context)
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
