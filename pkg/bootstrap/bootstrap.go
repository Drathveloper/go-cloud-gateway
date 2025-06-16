package bootstrap

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

const initializeErrMsg = "gateway initialization failed: %w"

// Initialize initializes the gateway.
//
// The gateway is initialized with the given options.
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
	globalFilters, err := config.NewGlobalFilters(opts.Config, filterFactory)
	if err != nil {
		return nil, fmt.Errorf(initializeErrMsg, err)
	}
	client, err := config.NewHTTPClient(opts.Config)
	if err != nil {
		return nil, fmt.Errorf(initializeErrMsg, err)
	}
	gwy := gateway.NewGateway(globalFilters, client)

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
		WriteTimeout:      opts.ServerOptions.WriteTimeout,
		ReadTimeout:       opts.ServerOptions.ReadTimeout,
		MaxHeaderBytes:    opts.ServerOptions.MaxHeaderBytes,
	}, nil
}
