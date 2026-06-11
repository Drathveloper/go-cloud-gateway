package gatewayhandler

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
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
	route := h.routes.FindMatching(request)
	if route == nil {
		http.Error(writer, ErrRouteNotFound.Error(), http.StatusNotFound)
		return
	}
	gwRequest := gateway.NewGatewayRequest(request)
	ctx, cancel := gateway.NewGatewayContext(request.Context(), route, gwRequest)
	defer cancel()
	if err := h.gateway.Do(ctx); err != nil {
		h.errHandler.Handle(ctx, err, writer)
		// Last-resort close: a Gateway implementation may have left the
		// backend body open when returning an error.
		if ctx.Response != nil && ctx.Response.BodyReader != nil {
			_ = ctx.Response.BodyReader.Close()
		}
		return
	}
	h.writeResponse(writer, ctx)
	gateway.ReleaseGatewayContext(ctx)
}

func (h *GatewayHandler) writeResponse(writer http.ResponseWriter, ctx *gateway.Context) {
	response := ctx.Response
	defer response.BodyReader.Close() //nolint:errcheck
	// The backend hop-by-hop headers belong to the gateway-backend connection:
	// forwarding e.g. its "Connection: close" would tear down the client keep-alive.
	shared.RemoveHopByHopHeaders(response.Headers)
	shared.WriteHeader(writer, response.Headers)
	// Set after copying the backend headers: filters may have changed the body, so
	// the buffered length is authoritative over any backend Content-Length. With an
	// unknown length the server picks the transfer encoding itself (chunked on
	// HTTP/1.1); setting Transfer-Encoding by hand is invalid on HTTP/2.
	if length := response.BodyReader.Len(); length >= 0 {
		writer.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	} else {
		writer.Header().Del("Content-Length")
	}
	writer.WriteHeader(response.Status)
	if _, err := io.Copy(writer, response.BodyReader); err != nil {
		ctx.Logger.Warn("copying backend response to client failed", "error", err)
		// The status and part of the body may already be on the wire: abort the
		// connection so the client sees the truncation instead of a chunked
		// response that terminates cleanly and looks complete.
		panic(http.ErrAbortHandler)
	}
}
