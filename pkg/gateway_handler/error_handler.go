package gateway_handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

type ErrorHandler interface {
	Handle(logger *slog.Logger, err error, w http.ResponseWriter)
}

type ErrorHandlerFunc func(logger *slog.Logger, err error, w http.ResponseWriter)

func (f ErrorHandlerFunc) Handle(logger *slog.Logger, err error, w http.ResponseWriter) {
	f(logger, err, w)
}

func BaseErrorHandler() ErrorHandlerFunc {
	return func(logger *slog.Logger, err error, writer http.ResponseWriter) {
		if err == nil {
			return
		}
		switch {
		case errors.Is(err, ErrRouteNotFound):
			logger.Info("route not found")
			http.Error(writer, "404 Route Not Found", http.StatusNotFound)
		case errors.Is(err, context.DeadlineExceeded):
			logger.Error("request timeout", "error", err)
			http.Error(writer, "", http.StatusBadGateway)
		case errors.Is(err, gateway.ErrHTTP):
			logger.Error("http request failed", "error", err)
			http.Error(writer, "", http.StatusBadGateway)
		default:
			logger.Error("unexpected error", "error", err)
			http.Error(writer, "", http.StatusInternalServerError)
		}
	}
}
