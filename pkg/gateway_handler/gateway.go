package gateway_handler

import (
	"bytes"
	"errors"
	"github.com/drathveloper/go-cloud-gateway/pkg/common"
	"io"
	"log/slog"
	"net/http"

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

func (h *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := slog.Default()
	route := h.routes.FindMatching(r)
	if route == nil {
		h.errHandler.Handle(logger, ErrRouteNotFound, w)
		return
	}
	logger = logger.With("routeId", route.ID)
	gwRequest, err := gateway.NewGatewayRequest(r)
	if err != nil {
		h.errHandler.Handle(logger, err, w)
		return
	}
	r = nil
	ctx, cancel := gateway.NewGatewayContext(route, gwRequest, logger)
	defer cancel()
	if err = h.gateway.Do(ctx); err != nil {
		h.errHandler.Handle(ctx.Logger, err, w)
		return
	}
	for k, vv := range ctx.Response.Headers {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	common.WriteHeader(w, ctx.Response.Headers)
	w.WriteHeader(ctx.Response.Status)
	_, _ = io.Copy(w, bytes.NewReader(ctx.Response.Body))
}
