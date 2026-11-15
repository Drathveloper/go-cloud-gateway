package bootstrap

import (
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

const initializeErrMsg = "gateway initialization failed: %w"

// routeTimeoutGrace is the margin the server read and write timeouts keep over the
// longest route timeout, so that the route timeout is the one that expires first and its
// response can still be written.
const routeTimeoutGrace = 5 * time.Second

// Initialize initializes the gateway.
//
// The gateway is initialized with the given options.
//
// Custom predicates and filters are only known to the gateway being initialized: the
// global predicate.BuilderRegistry and filter.BuilderRegistry are left untouched.
//
// The config is validated with config.Validate before anything is built, whether it was
// read by a reader or built in code. An invalid or nil config is an error.
//
// The server read and write timeouts are raised when they do not cover the longest route
// timeout: see ServerOpts.ReadTimeout and ServerOpts.WriteTimeout.
func Initialize(opts *Options) (*http.Server, error) {
	logger := opts.Logger
	if logger == nil {
		// Resolved here and not when the options are built, so that a default logger
		// installed in between is the one used.
		logger = slog.Default()
	}
	// A config read by a reader has been validated already, one built in code has not.
	if err := config.Validate(opts.Config); err != nil {
		return nil, fmt.Errorf(initializeErrMsg, err)
	}
	routes, err := config.NewRoutes(
		opts.Config, newPredicateFactory(opts.CustomPredicates), newFilterFactory(opts.CustomFilters), logger)
	if err != nil {
		return nil, fmt.Errorf(initializeErrMsg, err)
	}
	client, err := config.NewHTTPClient(opts.Config)
	if err != nil {
		return nil, fmt.Errorf(initializeErrMsg, err)
	}
	gwy := gateway.NewGateway(client)

	// A nil not found handler is ignored by the option, which keeps the default one.
	gatewayHandler := gatewayhandler.NewGatewayHandler(
		gwy, routes, opts.GatewayErrorHandler, gatewayhandler.WithNotFoundHandler(opts.NotFoundHandler))

	mux := http.NewServeMux()
	for _, customHandler := range opts.ServerOptions.CustomHandlers {
		mux.Handle(customHandler.Method+" "+customHandler.Path, customHandler.Handler)
	}
	mux.Handle("/", gatewayHandler)

	server := &http.Server{
		Handler:           mux,
		Addr:              fmt.Sprintf(":%d", opts.ServerOptions.Port),
		ReadHeaderTimeout: opts.ServerOptions.ReadHeaderTimeout,
		IdleTimeout:       opts.ServerOptions.IdleTimeout,
		WriteTimeout:      coverRouteTimeouts(opts.ServerOptions.WriteTimeout, routes),
		ReadTimeout:       opts.ServerOptions.ReadTimeout,
		MaxHeaderBytes:    opts.ServerOptions.MaxHeaderBytes,
	}
	raiseReadTimeout(server, routes)
	return server, nil
}

// newPredicateFactory returns a factory that knows the built-in predicates and the given
// custom ones. The custom builders are registered in a copy of the global registry:
// registered in the global one they would leak into every other gateway of the process,
// and one named like a built-in predicate would replace it for all of them.
func newPredicateFactory(customPredicates []CustomPredicate) *predicate.Factory {
	registry := maps.Clone(predicate.BuilderRegistry)
	for _, customPredicate := range customPredicates {
		registry.Register(customPredicate.Name, customPredicate.Builder)
	}
	return predicate.NewFactory(registry)
}

// newFilterFactory returns a factory that knows the built-in filters and the given custom
// ones. See newPredicateFactory for why the global registry is copied.
func newFilterFactory(customFilters []CustomFilter) *filter.Factory {
	registry := maps.Clone(filter.BuilderRegistry)
	for _, customFilter := range customFilters {
		registry.Register(customFilter.Name, customFilter.Builder)
	}
	return filter.NewFactory(registry)
}

// coverRouteTimeouts returns the read or write timeout the server must run with. Both
// are deadlines for a whole request or response and start counting before the route
// timeout does, so when one of them is not the longer of the two the route timeout is
// never reached: an expired read timeout cuts the request body the gateway is still
// streaming to the backend, and an expired write timeout closes the connection before
// the gateway can answer 504 Gateway Timeout. A non-positive value means no timeout and
// is left untouched.
func coverRouteTimeouts(configured time.Duration, routes gateway.Routes) time.Duration {
	if configured <= 0 {
		return configured
	}
	for i := range routes {
		configured = max(configured, routes[i].Timeout+routeTimeoutGrace)
	}
	return configured
}

// raiseReadTimeout makes the read timeout of the server cover the route timeouts.
// net/http falls back to the read timeout for a header or idle timeout left at zero:
// those are pinned to the configured read timeout, so that giving request bodies more
// time does not also give it to slow headers and idle connections.
func raiseReadTimeout(server *http.Server, routes gateway.Routes) {
	configured := server.ReadTimeout
	server.ReadTimeout = coverRouteTimeouts(configured, routes)
	if server.ReadTimeout == configured {
		return
	}
	if server.ReadHeaderTimeout == 0 {
		server.ReadHeaderTimeout = configured
	}
	if server.IdleTimeout == 0 {
		server.IdleTimeout = configured
	}
}
