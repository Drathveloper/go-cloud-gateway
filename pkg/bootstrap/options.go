package bootstrap

import (
	"net/http"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
)

// Options is the options for the initialization of the gateway.
type Options struct {
	Config              *config.Config
	CustomFilters       []CustomFilter
	CustomPredicates    []CustomPredicate
	GatewayErrorHandler gatewayhandler.ErrorHandler
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
