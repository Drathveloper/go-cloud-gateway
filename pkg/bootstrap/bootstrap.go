package bootstrap

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

const initializeErrMsg = "gateway initialization failed: %w"

// writeTimeoutGrace is the margin the server write timeout keeps over the longest route
// timeout, so that the response to a timed out route can still be written.
const writeTimeoutGrace = 5 * time.Second

// Initialize initializes the gateway.
//
// The gateway is initialized with the given options.
//
// The server write timeout is raised when it does not cover the longest route timeout:
// see ServerOpts.WriteTimeout.
func Initialize(opts *Options) (*http.Server, error) {
	for _, customPredicate := range opts.CustomPredicates {
		predicate.BuilderRegistry.Register(customPredicate.Name, customPredicate.Builder)
	}
	for _, customFilter := range opts.CustomFilters {
		filter.BuilderRegistry.Register(customFilter.Name, customFilter.Builder)
	}
	filterFactory := filter.NewFactory(filter.BuilderRegistry)
	predFactory := predicate.NewFactory(predicate.BuilderRegistry)
	routes, err := config.NewRoutes(opts.Config, predFactory, filterFactory, slog.Default())
	if err != nil {
		return nil, fmt.Errorf(initializeErrMsg, err)
	}
	client, err := config.NewHTTPClient(opts.Config)
	if err != nil {
		return nil, fmt.Errorf(initializeErrMsg, err)
	}
	gwy := gateway.NewGateway(client)

	gatewayHandler := gatewayhandler.NewGatewayHandler(gwy, routes, opts.GatewayErrorHandler)

	mux := http.NewServeMux()
	for _, customHandler := range opts.ServerOptions.CustomHandlers {
		mux.Handle(customHandler.Method+" "+customHandler.Path, customHandler.Handler)
	}
	mux.Handle("/", gatewayHandler)

	return &http.Server{
		Handler:           mux,
		Addr:              fmt.Sprintf(":%d", opts.ServerOptions.Port),
		ReadHeaderTimeout: opts.ServerOptions.ReadHeaderTimeout,
		IdleTimeout:       opts.ServerOptions.IdleTimeout,
		WriteTimeout:      serverWriteTimeout(opts.ServerOptions.WriteTimeout, routes),
		ReadTimeout:       opts.ServerOptions.ReadTimeout,
		MaxHeaderBytes:    opts.ServerOptions.MaxHeaderBytes,
	}, nil
}

// serverWriteTimeout returns the write timeout the server must run with. The server
// write timeout starts counting before the route timeout does, so when it is not the
// longer of the two the connection is closed before the gateway can answer 504 Gateway
// Timeout, and the route timeout is never reached. A non-positive value means no write
// timeout and is left untouched.
func serverWriteTimeout(configured time.Duration, routes gateway.Routes) time.Duration {
	if configured <= 0 {
		return configured
	}
	for i := range routes {
		configured = max(configured, routes[i].Timeout+writeTimeoutGrace)
	}
	return configured
}
