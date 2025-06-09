package gateway_handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

var ErrRouteNotFound = errors.New("route not found")

type Gateway interface {
	Do(ctx *gateway.Context) error
}

type GatewayHandler struct {
	gateway    Gateway
	routes     gateway.Routes
	errHandler ErrorHandler
}

func NewGatewayHandler(
	gateway Gateway,
	routes gateway.Routes,
	errHandler ErrorHandler) *GatewayHandler {
	return &GatewayHandler{
		gateway:    gateway,
		routes:     routes,
		errHandler: errHandler,
	}
}

func (h *GatewayHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	logger := slog.Default()
	route := h.routes.FindMatching(request)
	if route == nil {
		h.errHandler.Handle(logger, ErrRouteNotFound, writer)
		return
	}
	gwRequest, err := gateway.NewGatewayRequest(request)
	if err != nil {
		h.errHandler.Handle(logger, err, writer)
		return
	}
	ctx, cancel := gateway.NewGatewayContext(route, gwRequest)
	defer cancel()
	if err = h.gateway.Do(ctx); err != nil {
		h.errHandler.Handle(ctx.Logger, err, writer)
		return
	}
	common.WriteHeader(writer, ctx.Response.Headers)
	writer.WriteHeader(ctx.Response.Status)
	if len(ctx.Response.Body) > 0 {
		_, _ = writer.Write(ctx.Response.Body)
	}
	gateway.ReleaseGatewayContext(ctx)
}
