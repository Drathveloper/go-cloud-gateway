package gatewayhandler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

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

// statusClientClosedRequest is the de facto status code (nginx convention) for requests
// aborted by the client. The client is gone, so it is only seen in logs and metrics.
const statusClientClosedRequest = 499

// BaseErrorHandler is the base error handler. It will handle the following errors:
// 1. context.DeadlineExceeded: the request timeout. It will return a 504 Gateway Timeout.
// 2. context.Canceled: the client closed the request. It will return a 499 Client Closed Request.
// 3. gateway.ErrHTTP: the gateway http request to backend failed. It will return 502 Bad Gateway.
// 4. filter.ErrRateLimitExceeded: the rate limit exceeded. It will return 429 Too Many Requests, with a Retry-After
// header when the error is a filter.RateLimitExceededError that knows how long the client has to wait.
// 5. gateway.ErrCircuitBreaker: the circuit breaker is open. It will return 503 Service Unavailable.
// 6. any other error: unexpected error. It will return a 500 Internal Server Error.
// If the error is nil, it will do nothing.
func BaseErrorHandler() ErrorHandlerFunc {
	return func(ctx *gateway.Context, err error, writer http.ResponseWriter) {
		if err == nil {
			return
		}
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			ctx.Logger.Error("request timeout", "error", err)
			http.Error(writer, "", http.StatusGatewayTimeout)
		case errors.Is(err, context.Canceled):
			ctx.Logger.Warn("client closed request", "error", err)
			http.Error(writer, "", statusClientClosedRequest)
		case errors.Is(err, gateway.ErrHTTP):
			ctx.Logger.Error("http request failed", "error", err)
			http.Error(writer, "", http.StatusBadGateway)
		case errors.Is(err, filter.ErrRateLimitExceeded):
			// A rejection is the limiter working as intended, and under abuse there is one
			// per request: at error level they would bury the errors that need attention.
			ctx.Logger.Warn("rate limit exceeded", "error", err)
			setRetryAfter(writer.Header(), err)
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

// setRetryAfter sets the Retry-After header when err tells how long the client has to
// wait. Without it a well-behaved client cannot know when to come back. The header is
// left out when the wait is not known: a made up value would be worse than none.
func setRetryAfter(header http.Header, err error) {
	rateLimitErr, ok := errors.AsType[*filter.RateLimitExceededError](err)
	if !ok || rateLimitErr.RetryAfter <= 0 {
		return
	}
	// The header takes whole seconds. Rounding down would call the client back before a
	// token is available, and zero would tell it to retry at once.
	seconds := int64(rateLimitErr.RetryAfter / time.Second)
	if rateLimitErr.RetryAfter%time.Second != 0 {
		seconds++
	}
	header.Set("Retry-After", strconv.FormatInt(seconds, 10))
}
