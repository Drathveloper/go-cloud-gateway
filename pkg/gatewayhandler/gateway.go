package gatewayhandler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// ErrRouteNotFound is the error returned when no route matched the request.
var ErrRouteNotFound = errors.New("route not found")

// Gateway is the interface for the gateway.
// It is used to handle the request and return the response.
// The gateway context is used to store the request, response, contextual logging and other information.
type Gateway interface {
	Do(ctx *gateway.Context) error
}

// GatewayHandler is the handler for the gateway.
// It is used to handle the request, search the appropriate route and handle the gateway the response from context.
type GatewayHandler struct {
	gateway    Gateway
	errHandler ErrorHandler
	routes     gateway.Routes
}

// NewGatewayHandler creates a new gateway handler.
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

// ServeHTTP is the entrypoint for all requests to the gateway.
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
