package bootstrap

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
)

// Options is the options for the initialization of the gateway.
//
// Logger is the logger the gateway logs with. Each route logs through a child of it that
// carries the route id in the "route" attribute. Nil means slog.Default(), resolved when
// Initialize runs.
//
// NotFoundHandler answers the requests that match no route. Nil means the default
// handler: see gatewayhandler.WithNotFoundHandler.
//
// CustomFilters and CustomPredicates are only registered for the gateway built from
// these options. One named like a built-in filter or predicate replaces it for that
// gateway alone.
type Options struct {
	Config              *config.Config
	Logger              *slog.Logger
	GatewayErrorHandler gatewayhandler.ErrorHandler
	NotFoundHandler     http.Handler
	CustomFilters       []CustomFilter
	CustomPredicates    []CustomPredicate
	ServerOptions       ServerOpts
}

// ServerOpts is the options for the server.
//
// ReadTimeout and WriteTimeout are raised by Initialize to the longest route timeout
// plus a margin when they are positive and shorter than that; a value that disables the
// timeout is kept. A read timeout that expires before a route timeout cuts the request
// body while it is still being sent to the backend, and a write timeout that expires
// before it closes the connection before the gateway can answer.
//
// The write timeout does not apply to streaming routes. The read timeout does: the
// request body of a streaming route is still bounded by it.
//
// ReadHeaderTimeout and IdleTimeout fall back to ReadTimeout when they are zero. The
// fallback is the configured ReadTimeout, not the raised one.
//
// A zero or negative timeout means no timeout. OptionsBuilder.WithServerOptions cannot
// set a field to zero, because it reads zero as "keep the current value": use a negative
// value there to disable a timeout.
type ServerOpts struct {
	CustomHandlers    []CustomHandler
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	WriteTimeout      time.Duration
	ReadTimeout       time.Duration
	Port              int
	MaxHeaderBytes    int
}

// CustomHandler is a custom http handler.
type CustomHandler struct {
	Handler http.Handler
	Method  string
	Path    string
}

// CustomFilter is a custom filter.
type CustomFilter struct {
	Builder gateway.FilterBuilder
	Name    string
}

// CustomPredicate is a custom predicate.
type CustomPredicate struct {
	Builder gateway.PredicateBuilder
	Name    string
}
