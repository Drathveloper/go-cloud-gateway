package gatewayhandler

import (
	"context"
	"errors"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// ErrorHandler is the interface for the error handler.
type ErrorHandler interface {
	// Handle handles the error. It will be in charge of writing the response to the writer.
	Handle(ctx *gateway.Context, err error, w http.ResponseWriter)
}

// ErrorHandlerFunc is the function type for the error handler.
type ErrorHandlerFunc func(ctx *gateway.Context, err error, w http.ResponseWriter)

// Handle calls the ErrorHandler Handle function.
func (f ErrorHandlerFunc) Handle(ctx *gateway.Context, err error, w http.ResponseWriter) {
	f(ctx, err, w)
}

// BaseErrorHandler is the base error handler. It will handle the following errors:
// 1. ErrRouteNotFound: no route matched the request. It will return a 404 Not Found.
// 2. context.DeadlineExceeded: the request timeout. It will return a 502 Bad Gateway.
// 3. gateway.ErrHTTP: the gateway http request to backend failed. It will return 502 Bad Gateway.
// 4. filter.ErrRateLimitExceeded: the rate limit exceeded. It will return 429 Too Many Requests.
// 4. any other error: unexpected error. It will return a 500 Internal Server Error.
// If the error is not one of the above, it will log the error and return a 500 Internal Server Error.
// If the error is nil, it will do nothing.
func BaseErrorHandler() ErrorHandlerFunc {
	return func(ctx *gateway.Context, err error, writer http.ResponseWriter) {
		if err == nil {
			return
		}
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			ctx.Logger.Error("request timeout", "error", err)
			http.Error(writer, "", http.StatusBadGateway)
		case errors.Is(err, gateway.ErrHTTP):
			ctx.Logger.Error("http request failed", "error", err)
			http.Error(writer, "", http.StatusBadGateway)
		case errors.Is(err, filter.ErrRateLimitExceeded):
			ctx.Logger.Error("rate limit exceeded", "error", err)
			http.Error(writer, "", http.StatusTooManyRequests)
		case errors.Is(err, gateway.ErrCircuitBreaker):
			ctx.Logger.Error("circuit breaker is open", "error", err)
			http.Error(writer, "", http.StatusServiceUnavailable)
		default:
			ctx.Logger.Error("unexpected error", "error", err)
			http.Error(writer, "", http.StatusInternalServerError)
		}
	}
}
