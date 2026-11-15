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

// routeTimeoutGrace is the margin the server read and write timeouts keep over the
// longest route timeout, so that the route timeout is the one that expires first and its
// response can still be written.
const routeTimeoutGrace = 5 * time.Second

// Initialize initializes the gateway.
//
// The gateway is initialized with the given options.
//
// The server read and write timeouts are raised when they do not cover the longest route
// timeout: see ServerOpts.ReadTimeout and ServerOpts.WriteTimeout.
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
