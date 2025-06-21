package gatewayhandler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

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
	gwRequest := gateway.NewGatewayRequest(request)
	ctx, cancel := gateway.NewGatewayContext(route, gwRequest)
	defer cancel()
	if err := h.gateway.Do(ctx); err != nil {
		h.errHandler.Handle(ctx.Logger, err, writer)
		return
	}

	if ctx.Response.BodyReader.Len() == -1 {
		writer.Header().Set("Transfer-Encoding", "chunked")
	} else {
		writer.Header().Set("Content-Length", strconv.FormatInt(ctx.Response.BodyReader.Len(), 10))
	}
	h.writeResponse(writer, ctx.Response)
	gateway.ReleaseGatewayContext(ctx)
}

func (h *GatewayHandler) writeResponse(writer http.ResponseWriter, response *gateway.Response) {
	common.WriteHeader(writer, response.Headers)
	writer.WriteHeader(response.Status)
	_, _ = io.Copy(writer, response.BodyReader)
	_ = response.BodyReader.Close()
}
