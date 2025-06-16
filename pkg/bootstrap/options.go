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
