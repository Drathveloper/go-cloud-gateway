package gatewayhandler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// ErrorHandler is the interface for the error handler.
type ErrorHandler interface {
	// Handle handles the error. It will be in charge of writing the response to the writer.
	Handle(logger *slog.Logger, err error, w http.ResponseWriter)
}

// ErrorHandlerFunc is the function type for the error handler.
type ErrorHandlerFunc func(logger *slog.Logger, err error, w http.ResponseWriter)

// Handle calls the ErrorHandler Handle function.
func (f ErrorHandlerFunc) Handle(logger *slog.Logger, err error, w http.ResponseWriter) {
	f(logger, err, w)
}

// BaseErrorHandler is the base error handler. It will handle the following errors:
// 1. ErrRouteNotFound: no route matched the request. It will return a 404 Not Found.
// 2. context.DeadlineExceeded: the request timeout. It will return a 502 Bad Gateway.
// 3. gateway.ErrHTTP: the gateway http request to backend failed. It will return a 502 Bad Gateway.
// 4. any other error: unexpected error. It will return a 500 Internal Server Error.
// If the error is not one of the above, it will log the error and return a 500 Internal Server Error.
// If the error is nil, it will do nothing.
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
