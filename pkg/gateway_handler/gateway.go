package gateway_handler

import (
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"time"

	"github.com/google/uuid"

	"gateway/pkg/gateway"
)

var ErrRouteNotFound = errors.New("route not found")

type Gateway interface {
	Do(ctx *gateway.Context) error
}

type GatewayHandler struct {
	gateway    Gateway
	routes     gateway.Routes
	logger     *slog.Logger
	timeout    time.Duration
	errHandler ErrorHandler
}

func NewGatewayHandler(
	gateway Gateway,
	routes gateway.Routes,
	logger *slog.Logger,
	timeout time.Duration,
	errHandler ErrorHandler) *GatewayHandler {
	return &GatewayHandler{
		gateway:    gateway,
		routes:     routes,
		logger:     logger,
		timeout:    timeout,
		errHandler: errHandler,
	}
}

func (h *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With("requestId", uuid.New().String())
	route := h.routes.FindMatching(r)
	if route == nil {
		h.errHandler.Handle(logger, ErrRouteNotFound, w)
		return
	}
	gwRequest, err := gateway.NewGatewayRequest(r)
	if err != nil {
		h.errHandler.Handle(logger, err, w)
		return
	}
	timeout := h.calculateTimeout(route)
	ctx, cancel := gateway.NewGatewayContext(route, gwRequest, logger, timeout)
	defer cancel()
	if err = h.gateway.Do(ctx); err != nil {
		h.errHandler.Handle(ctx.Logger, err, w)
		return
	}
	maps.Copy(w.Header(), ctx.Response.Headers)
	w.WriteHeader(ctx.Response.Status)
	_, _ = w.Write(ctx.Response.Body)
}

func (h *GatewayHandler) calculateTimeout(r *gateway.Route) time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return h.timeout
}
