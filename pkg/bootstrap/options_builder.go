package bootstrap

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
)

const defaultReadHeaderTimeout = 2 * time.Second
const defaultIdleTimeout = 60 * time.Second
const defaultWriteTimeout = 10 * time.Second
const defaultReadTimeout = 10 * time.Second
const defaultPort = 8000
const defaultMaxHeaderBytes = 1 << 20

// OptionsBuilder is the builder for the initialization options.
type OptionsBuilder struct {
	config             *config.Config
	logger             *slog.Logger
	customErrorHandler gatewayhandler.ErrorHandler
	notFoundHandler    http.Handler
	customFilters      []CustomFilter
	customPredicates   []CustomPredicate
	serverOptions      ServerOpts
}

// NewOptionsBuilder creates a new option builder.
func NewOptionsBuilder(config *config.Config) *OptionsBuilder {
	return &OptionsBuilder{
		config:           config,
		customFilters:    []CustomFilter{},
		customPredicates: []CustomPredicate{},
		serverOptions: ServerOpts{
			CustomHandlers:    []CustomHandler{},
			ReadHeaderTimeout: defaultReadHeaderTimeout,
			IdleTimeout:       defaultIdleTimeout,
			WriteTimeout:      defaultWriteTimeout,
			ReadTimeout:       defaultReadTimeout,
			Port:              defaultPort,
			MaxHeaderBytes:    defaultMaxHeaderBytes,
		},
		customErrorHandler: gatewayhandler.BaseErrorHandler(),
	}
}

// WithCustomFilters sets the custom filters.
func (b *OptionsBuilder) WithCustomFilters(filters ...CustomFilter) *OptionsBuilder {
	b.customFilters = filters
	return b
}

// WithCustomPredicates sets the custom predicates.
func (b *OptionsBuilder) WithCustomPredicates(predicates ...CustomPredicate) *OptionsBuilder {
	b.customPredicates = predicates
	return b
}

// WithServerOptions sets the server options.
//
// The server options are merged with the default options.
func (b *OptionsBuilder) WithServerOptions(opts ServerOpts) *OptionsBuilder {
	if opts.ReadHeaderTimeout != 0 {
		b.serverOptions.ReadHeaderTimeout = opts.ReadHeaderTimeout
	}
	if opts.IdleTimeout != 0 {
		b.serverOptions.IdleTimeout = opts.IdleTimeout
	}
	if opts.WriteTimeout != 0 {
		b.serverOptions.WriteTimeout = opts.WriteTimeout
	}
	if opts.ReadTimeout != 0 {
		b.serverOptions.ReadTimeout = opts.ReadTimeout
	}
	if opts.Port != 0 {
		b.serverOptions.Port = opts.Port
	}
	if opts.MaxHeaderBytes != 0 {
		b.serverOptions.MaxHeaderBytes = opts.MaxHeaderBytes
	}
	return b
}

// WithCustomHandlers sets the custom handlers.
func (b *OptionsBuilder) WithCustomHandlers(handlers ...CustomHandler) *OptionsBuilder {
	b.serverOptions.CustomHandlers = handlers
	return b
}

// WithErrorHandler sets the error handler.
func (b *OptionsBuilder) WithErrorHandler(errorHandler gatewayhandler.ErrorHandler) *OptionsBuilder {
	b.customErrorHandler = errorHandler
	return b
}

// WithLogger sets the logger the gateway logs with.
//
// Nil, the default, means slog.Default(), resolved when Initialize runs.
func (b *OptionsBuilder) WithLogger(logger *slog.Logger) *OptionsBuilder {
	b.logger = logger
	return b
}

// WithNotFoundHandler sets the handler that answers the requests that match no route.
//
// Nil, the default, keeps the default handler: see gatewayhandler.WithNotFoundHandler.
func (b *OptionsBuilder) WithNotFoundHandler(handler http.Handler) *OptionsBuilder {
	b.notFoundHandler = handler
	return b
}

// Build builds the options.
func (b *OptionsBuilder) Build() *Options {
	return &Options{
		Config:              b.config,
		Logger:              b.logger,
		NotFoundHandler:     b.notFoundHandler,
		CustomFilters:       b.customFilters,
		CustomPredicates:    b.customPredicates,
		GatewayErrorHandler: b.customErrorHandler,
		ServerOptions:       b.serverOptions,
	}
}
