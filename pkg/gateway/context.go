package gateway

import (
	"context"
	"log/slog"
	"time"
)

type Context struct {
	Request    *Request
	Response   *Response
	Route      *Route
	Attributes map[string]any
	Logger     *slog.Logger
	context.Context
}

func NewGatewayContext(
	route *Route,
	req *Request,
	logger *slog.Logger,
	timeout time.Duration) (*Context, context.CancelFunc) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	return &Context{
		Request:    req,
		Response:   nil,
		Route:      route,
		Attributes: make(map[string]any),
		Logger:     logger,
		Context:    ctx,
	}, cancelFunc
}
