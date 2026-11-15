package bootstrap_test

import (
	"log/slog"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
)

func TestOptionsBuilder_Build(t *testing.T) {
	dummyFilterBuilder := gateway.FilterBuilderFunc(func(_ map[string]any) (gateway.Filter, error) {
		return nil, nil //nolint:nilnil
	})
	dummyPredicateBuilder := gateway.PredicateBuilderFunc(func(_ map[string]any) (gateway.Predicate, error) {
		return nil, nil //nolint:nilnil
	})
	dummyHTTPHandler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})
	dummyConfig := &config.Config{}

	tests := []struct {
		name             string
		cfg              *config.Config
		customFilters    []bootstrap.CustomFilter
		customPredicates []bootstrap.CustomPredicate
		customHandlers   []bootstrap.CustomHandler
		serverOptions    *bootstrap.ServerOpts
		expectedOpts     bootstrap.Options
	}{
		{
			name: "build should succeed when only config provided",
			cfg:  dummyConfig,
			expectedOpts: bootstrap.Options{
				Config:              dummyConfig,
				CustomFilters:       make([]bootstrap.CustomFilter, 0),
				CustomPredicates:    make([]bootstrap.CustomPredicate, 0),
				GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
				ServerOptions: bootstrap.ServerOpts{
					CustomHandlers:    make([]bootstrap.CustomHandler, 0),
					ReadHeaderTimeout: 2 * time.Second,
					IdleTimeout:       60 * time.Second,
					WriteTimeout:      10 * time.Second,
					ReadTimeout:       10 * time.Second,
					Port:              8000,
					MaxHeaderBytes:    1048576,
				},
			},
		},
		{
			name: "build should succeed when custom filters provided",
			cfg:  dummyConfig,
			customFilters: []bootstrap.CustomFilter{
				{
					Builder: dummyFilterBuilder,
					Name:    "filter1",
				},
			},
			expectedOpts: bootstrap.Options{
				Config: dummyConfig,
				CustomFilters: []bootstrap.CustomFilter{
					{
						Builder: dummyFilterBuilder,
						Name:    "filter1",
					},
				},
				CustomPredicates:    make([]bootstrap.CustomPredicate, 0),
				GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
				ServerOptions: bootstrap.ServerOpts{
					CustomHandlers:    make([]bootstrap.CustomHandler, 0),
					ReadHeaderTimeout: 2 * time.Second,
					IdleTimeout:       60 * time.Second,
					WriteTimeout:      10 * time.Second,
					ReadTimeout:       10 * time.Second,
					Port:              8000,
					MaxHeaderBytes:    1048576,
				},
			},
		},
		{
			name:          "build should succeed when custom predicates provided",
			cfg:           dummyConfig,
			customFilters: []bootstrap.CustomFilter{},
			customPredicates: []bootstrap.CustomPredicate{
				{
					Builder: dummyPredicateBuilder,
					Name:    "predicate1",
				},
			},
			expectedOpts: bootstrap.Options{
				Config:        dummyConfig,
				CustomFilters: []bootstrap.CustomFilter{},
				CustomPredicates: []bootstrap.CustomPredicate{
					{
						Builder: dummyPredicateBuilder,
						Name:    "predicate1",
					},
				},
				GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
				ServerOptions: bootstrap.ServerOpts{
					CustomHandlers:    make([]bootstrap.CustomHandler, 0),
					ReadHeaderTimeout: 2 * time.Second,
					IdleTimeout:       60 * time.Second,
					WriteTimeout:      10 * time.Second,
					ReadTimeout:       10 * time.Second,
					Port:              8000,
					MaxHeaderBytes:    1048576,
				},
			},
		},
		{
			name:             "build should succeed when custom handlers provided",
			cfg:              dummyConfig,
			customFilters:    []bootstrap.CustomFilter{},
			customPredicates: []bootstrap.CustomPredicate{},
			customHandlers: []bootstrap.CustomHandler{
				{
					Handler: dummyHTTPHandler,
					Method:  "GET",
					Path:    "/health",
				},
			},
			expectedOpts: bootstrap.Options{
				Config:              dummyConfig,
				CustomFilters:       []bootstrap.CustomFilter{},
				CustomPredicates:    []bootstrap.CustomPredicate{},
				GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
				ServerOptions: bootstrap.ServerOpts{
					CustomHandlers: []bootstrap.CustomHandler{
						{
							Handler: dummyHTTPHandler,
							Method:  "GET",
							Path:    "/health",
						},
					},
					ReadHeaderTimeout: 2 * time.Second,
					IdleTimeout:       60 * time.Second,
					WriteTimeout:      10 * time.Second,
					ReadTimeout:       10 * time.Second,
					Port:              8000,
					MaxHeaderBytes:    1048576,
				},
			},
		},
		{
			name:             "build should succeed when custom server options read header timeout provided",
			cfg:              dummyConfig,
			customFilters:    []bootstrap.CustomFilter{},
			customPredicates: []bootstrap.CustomPredicate{},
			customHandlers:   []bootstrap.CustomHandler{},
			serverOptions: &bootstrap.ServerOpts{
				ReadHeaderTimeout: 3 * time.Second,
				IdleTimeout:       0,
				WriteTimeout:      0,
				ReadTimeout:       0,
				Port:              0,
				MaxHeaderBytes:    0,
			},
			expectedOpts: bootstrap.Options{
				Config:              dummyConfig,
				CustomFilters:       []bootstrap.CustomFilter{},
				CustomPredicates:    []bootstrap.CustomPredicate{},
				GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
				ServerOptions: bootstrap.ServerOpts{
					CustomHandlers:    []bootstrap.CustomHandler{},
					ReadHeaderTimeout: 3 * time.Second,
					IdleTimeout:       60 * time.Second,
					WriteTimeout:      10 * time.Second,
					ReadTimeout:       10 * time.Second,
					Port:              8000,
					MaxHeaderBytes:    1048576,
				},
			},
		},
		{
			name:             "build should succeed when custom server options idle timeout provided",
			cfg:              dummyConfig,
			customFilters:    []bootstrap.CustomFilter{},
			customPredicates: []bootstrap.CustomPredicate{},
			customHandlers:   []bootstrap.CustomHandler{},
			serverOptions: &bootstrap.ServerOpts{
				ReadHeaderTimeout: 0,
				IdleTimeout:       10 * time.Second,
				WriteTimeout:      0,
				ReadTimeout:       0,
				Port:              0,
				MaxHeaderBytes:    0,
			},
			expectedOpts: bootstrap.Options{
				Config:              dummyConfig,
				CustomFilters:       []bootstrap.CustomFilter{},
				CustomPredicates:    []bootstrap.CustomPredicate{},
				GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
				ServerOptions: bootstrap.ServerOpts{
					CustomHandlers:    []bootstrap.CustomHandler{},
					ReadHeaderTimeout: 2 * time.Second,
					IdleTimeout:       10 * time.Second,
					WriteTimeout:      10 * time.Second,
					ReadTimeout:       10 * time.Second,
					Port:              8000,
					MaxHeaderBytes:    1048576,
				},
			},
		},
		{
			name:             "build should succeed when custom server options write timeout provided",
			cfg:              dummyConfig,
			customFilters:    []bootstrap.CustomFilter{},
			customPredicates: []bootstrap.CustomPredicate{},
			customHandlers:   []bootstrap.CustomHandler{},
			serverOptions: &bootstrap.ServerOpts{
				ReadHeaderTimeout: 0,
				IdleTimeout:       0,
				WriteTimeout:      1 * time.Second,
				ReadTimeout:       0,
				Port:              0,
				MaxHeaderBytes:    0,
			},
			expectedOpts: bootstrap.Options{
				Config:              dummyConfig,
				CustomFilters:       []bootstrap.CustomFilter{},
				CustomPredicates:    []bootstrap.CustomPredicate{},
				GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
				ServerOptions: bootstrap.ServerOpts{
					CustomHandlers:    []bootstrap.CustomHandler{},
					ReadHeaderTimeout: 2 * time.Second,
					IdleTimeout:       60 * time.Second,
					WriteTimeout:      1 * time.Second,
					ReadTimeout:       10 * time.Second,
					Port:              8000,
					MaxHeaderBytes:    1048576,
				},
			},
		},
		{
			name:             "build should succeed when custom server options read timeout provided",
			cfg:              dummyConfig,
			customFilters:    []bootstrap.CustomFilter{},
			customPredicates: []bootstrap.CustomPredicate{},
			customHandlers:   []bootstrap.CustomHandler{},
			serverOptions: &bootstrap.ServerOpts{
				ReadHeaderTimeout: 0,
				IdleTimeout:       0,
				WriteTimeout:      0,
				ReadTimeout:       1 * time.Second,
				Port:              0,
				MaxHeaderBytes:    0,
			},
			expectedOpts: bootstrap.Options{
				Config:              dummyConfig,
				CustomFilters:       []bootstrap.CustomFilter{},
				CustomPredicates:    []bootstrap.CustomPredicate{},
				GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
				ServerOptions: bootstrap.ServerOpts{
					CustomHandlers:    []bootstrap.CustomHandler{},
					ReadHeaderTimeout: 2 * time.Second,
					IdleTimeout:       60 * time.Second,
					WriteTimeout:      10 * time.Second,
					ReadTimeout:       1 * time.Second,
					Port:              8000,
					MaxHeaderBytes:    1048576,
				},
			},
		},
		{
			name:             "build should succeed when custom server options port provided",
			cfg:              dummyConfig,
			customFilters:    []bootstrap.CustomFilter{},
			customPredicates: []bootstrap.CustomPredicate{},
			customHandlers:   []bootstrap.CustomHandler{},
			serverOptions: &bootstrap.ServerOpts{
				ReadHeaderTimeout: 0,
				IdleTimeout:       0,
				WriteTimeout:      0,
				ReadTimeout:       0,
				Port:              1234,
				MaxHeaderBytes:    0,
			},
			expectedOpts: bootstrap.Options{
				Config:              dummyConfig,
				CustomFilters:       []bootstrap.CustomFilter{},
				CustomPredicates:    []bootstrap.CustomPredicate{},
				GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
				ServerOptions: bootstrap.ServerOpts{
					CustomHandlers:    []bootstrap.CustomHandler{},
					ReadHeaderTimeout: 2 * time.Second,
					IdleTimeout:       60 * time.Second,
					WriteTimeout:      10 * time.Second,
					ReadTimeout:       10 * time.Second,
					Port:              1234,
					MaxHeaderBytes:    1048576,
				},
			},
		},
		{
			name:             "build should succeed when custom server options max header bytes provided",
			cfg:              dummyConfig,
			customFilters:    []bootstrap.CustomFilter{},
			customPredicates: []bootstrap.CustomPredicate{},
			customHandlers:   []bootstrap.CustomHandler{},
			serverOptions: &bootstrap.ServerOpts{
				ReadHeaderTimeout: 0,
				IdleTimeout:       0,
				WriteTimeout:      0,
				ReadTimeout:       0,
				Port:              0,
				MaxHeaderBytes:    1234,
			},
			expectedOpts: bootstrap.Options{
				Config:              dummyConfig,
				CustomFilters:       []bootstrap.CustomFilter{},
				CustomPredicates:    []bootstrap.CustomPredicate{},
				GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
				ServerOptions: bootstrap.ServerOpts{
					CustomHandlers:    []bootstrap.CustomHandler{},
					ReadHeaderTimeout: 2 * time.Second,
					IdleTimeout:       60 * time.Second,
					WriteTimeout:      10 * time.Second,
					ReadTimeout:       10 * time.Second,
					Port:              8000,
					MaxHeaderBytes:    1234,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := bootstrap.NewOptionsBuilder(tt.cfg)

			if tt.customFilters != nil {
				builder.WithCustomFilters(tt.customFilters...)
			}
			if tt.customPredicates != nil {
				builder.WithCustomPredicates(tt.customPredicates...)
			}
			if tt.customHandlers != nil {
				builder.WithCustomHandlers(tt.customHandlers...)
			}
			if tt.serverOptions != nil {
				builder.WithServerOptions(*tt.serverOptions)
			}

			opts := builder.Build()

			if !reflect.DeepEqual(tt.expectedOpts.Config, opts.Config) {
				t.Errorf("expected %+v actual %+v", tt.expectedOpts, opts)
			}
			if !reflect.DeepEqual(len(tt.expectedOpts.CustomFilters), len(opts.CustomFilters)) {
				t.Errorf("expected %+v actual %+v", tt.expectedOpts, opts)
			}
			if !reflect.DeepEqual(len(tt.expectedOpts.CustomPredicates), len(opts.CustomPredicates)) {
				t.Errorf("expected %+v actual %+v", tt.expectedOpts, opts)
			}
			assertServerOpts(t, tt.expectedOpts.ServerOptions, opts.ServerOptions)
		})
	}
}

func TestOptionsBuilder_Build_LoggerAndNotFoundHandler(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	// A pointer, so that the handlers can be compared.
	notFoundHandler := http.NewServeMux()
	tests := []struct {
		configure               func(builder *bootstrap.OptionsBuilder)
		expectedLogger          *slog.Logger
		expectedNotFoundHandler http.Handler
		name                    string
	}{
		{
			name:                    "build should leave logger and not found handler unset when none provided",
			configure:               func(_ *bootstrap.OptionsBuilder) {},
			expectedLogger:          nil,
			expectedNotFoundHandler: nil,
		},
		{
			name: "build should succeed when logger provided",
			configure: func(builder *bootstrap.OptionsBuilder) {
				builder.WithLogger(logger)
			},
			expectedLogger:          logger,
			expectedNotFoundHandler: nil,
		},
		{
			name: "build should succeed when not found handler provided",
			configure: func(builder *bootstrap.OptionsBuilder) {
				builder.WithNotFoundHandler(notFoundHandler)
			},
			expectedLogger:          nil,
			expectedNotFoundHandler: notFoundHandler,
		},
		{
			name: "build should succeed when logger and not found handler provided",
			configure: func(builder *bootstrap.OptionsBuilder) {
				builder.WithLogger(logger).WithNotFoundHandler(notFoundHandler)
			},
			expectedLogger:          logger,
			expectedNotFoundHandler: notFoundHandler,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := bootstrap.NewOptionsBuilder(&config.Config{})
			tt.configure(builder)

			opts := builder.Build()

			if opts.Logger != tt.expectedLogger {
				t.Errorf("expected logger %v actual %v", tt.expectedLogger, opts.Logger)
			}
			if opts.NotFoundHandler != tt.expectedNotFoundHandler {
				t.Errorf("expected not found handler %v actual %v", tt.expectedNotFoundHandler, opts.NotFoundHandler)
			}
		})
	}
}

func assertServerOpts(t *testing.T, expectedOpts bootstrap.ServerOpts, actualOpts bootstrap.ServerOpts) {
	t.Helper()
	if expectedOpts.ReadHeaderTimeout != actualOpts.ReadHeaderTimeout {
		t.Errorf("expected %+v actual %+v", expectedOpts, actualOpts)
	}
	if expectedOpts.IdleTimeout != actualOpts.IdleTimeout {
		t.Errorf("expected %+v actual %+v", expectedOpts, actualOpts)
	}
	if expectedOpts.WriteTimeout != actualOpts.WriteTimeout {
		t.Errorf("expected %+v actual %+v", expectedOpts, actualOpts)
	}
	if expectedOpts.ReadTimeout != actualOpts.ReadTimeout {
		t.Errorf("expected %+v actual %+v", expectedOpts, actualOpts)
	}
	if expectedOpts.Port != actualOpts.Port {
		t.Errorf("expected %+v actual %+v", expectedOpts, actualOpts)
	}
	if expectedOpts.MaxHeaderBytes != actualOpts.MaxHeaderBytes {
		t.Errorf("expected %+v actual %+v", expectedOpts, actualOpts)
	}
}

func TestOptionsBuilder_WithServerOptions_CustomHandlers(t *testing.T) {
	health := bootstrap.CustomHandler{Handler: http.NotFoundHandler(), Method: http.MethodGet, Path: "/health"}
	ready := bootstrap.CustomHandler{Handler: http.NotFoundHandler(), Method: http.MethodGet, Path: "/ready"}
	tests := []struct {
		configure     func(builder *bootstrap.OptionsBuilder)
		name          string
		expectedPaths []string
	}{
		{
			name: "server options custom handlers should be taken",
			configure: func(builder *bootstrap.OptionsBuilder) {
				builder.WithServerOptions(bootstrap.ServerOpts{CustomHandlers: []bootstrap.CustomHandler{health}})
			},
			expectedPaths: []string{"/health"},
		},
		{
			name: "server options without custom handlers should keep the ones already set",
			configure: func(builder *bootstrap.OptionsBuilder) {
				builder.WithCustomHandlers(health).WithServerOptions(bootstrap.ServerOpts{Port: 9000})
			},
			expectedPaths: []string{"/health"},
		},
		{
			name: "server options custom handlers should replace the ones already set",
			configure: func(builder *bootstrap.OptionsBuilder) {
				builder.WithCustomHandlers(health).
					WithServerOptions(bootstrap.ServerOpts{CustomHandlers: []bootstrap.CustomHandler{ready}})
			},
			expectedPaths: []string{"/ready"},
		},
		{
			name: "custom handlers should replace the ones of the server options",
			configure: func(builder *bootstrap.OptionsBuilder) {
				builder.WithServerOptions(bootstrap.ServerOpts{CustomHandlers: []bootstrap.CustomHandler{health}}).
					WithCustomHandlers(ready)
			},
			expectedPaths: []string{"/ready"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := bootstrap.NewOptionsBuilder(&config.Config{})

			tt.configure(builder)

			handlers := builder.Build().ServerOptions.CustomHandlers
			paths := make([]string, 0, len(handlers))
			for _, handler := range handlers {
				paths = append(paths, handler.Path)
			}
			if !reflect.DeepEqual(tt.expectedPaths, paths) {
				t.Errorf("expected custom handler paths %v actual %v", tt.expectedPaths, paths)
			}
		})
	}
}
