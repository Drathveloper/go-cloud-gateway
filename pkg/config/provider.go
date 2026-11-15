package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/circuitbreaker"
	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/httpclient"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

// ErrInitializeMTLS is the error returned when the mTLS initialization failed.
var ErrInitializeMTLS = errors.New("failed to initialize mTLS")

// routeLogKey is the attribute every log line of a route carries with the route id.
const routeLogKey = "route"

// NewRoutes creates a new gateway route from the given config.
//
// Each route logs through a child of the given logger that carries the route id in the
// "route" attribute, and so does its circuit breaker. A nil logger means slog.Default().
func NewRoutes(
	cfg *Config,
	predicateFactory *predicate.Factory,
	filterFactory *filter.Factory,
	logger *slog.Logger) (gateway.Routes, error) {
	if logger == nil {
		// The route loggers are derived from this one, and deriving from a nil logger
		// panics: without the fallback, routes could no longer be built without a logger.
		logger = slog.Default()
	}
	return mapRoutesFromConfigToGateway(cfg.Gateway, predicateFactory, filterFactory, logger)
}

// NewHTTPClient creates a new http client from the given config.
// If any route has circuit breaker enabled, the http client will be wrapped with a circuit breaker client.
// Otherwise, the http client will be returned as is.
//
//nolint:ireturn
func NewHTTPClient(cfg *Config) (gateway.HTTPClient, error) {
	httpClient, err := buildHTTPClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build http client: %w", err)
	}
	if cfg != nil {
		for _, route := range cfg.Gateway.Routes {
			if route.CircuitBreaker.Enabled {
				return httpclient.NewCircuitBreakerHTTPClient(httpClient), nil
			}
		}
	}
	return httpClient, nil
}

//nolint:ireturn
func buildHTTPClient(cfg *Config) (gateway.HTTPClient, error) {
	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}
	transport := buildTransport(httpClientConfig(cfg), tlsConfig)
	// No client-wide timeout exists: it would cap the whole exchange including the body
	// copy, cutting long downloads and streams. Each request is bounded by the per-route
	// context deadline instead; pool.timeout only bounds dialing.
	return httpclient.NewTransportHTTPClient(transport), nil
}

// httpClientConfig returns the http client section of the config, or an empty one when
// the config or the section is missing, so that every setting falls back to its default.
func httpClientConfig(cfg *Config) HTTPClient {
	if cfg == nil || cfg.Gateway.HTTPClient == nil {
		return HTTPClient{}
	}
	return *cfg.Gateway.HTTPClient
}

func buildTLSConfig(cfg *Config) (*tls.Config, error) {
	if cfg != nil &&
		cfg.Gateway.HTTPClient != nil &&
		cfg.Gateway.HTTPClient.MTLS != nil &&
		*cfg.Gateway.HTTPClient.MTLS.Enabled {
		tlsConfig, err := buildMTLSConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to build mTLS config: %w", err)
		}
		return tlsConfig, nil
	}
	return buildDefaultTLSConfig(cfg), nil
}

func buildMTLSConfig(cfg *Config) (*tls.Config, error) {
	keyPair, err := tls.X509KeyPair([]byte(cfg.Gateway.HTTPClient.MTLS.Cert), []byte(cfg.Gateway.HTTPClient.MTLS.Key))
	if err != nil {
		return nil, fmt.Errorf("%w failed to load mTLS cert/key pair: %s", ErrInitializeMTLS, err.Error())
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM([]byte(cfg.Gateway.HTTPClient.MTLS.CA)) {
		return nil, ErrInitializeMTLS
	}
	return &tls.Config{
		InsecureSkipVerify: isInsecureSkipVerify(cfg), //nolint:gosec
		Certificates:       []tls.Certificate{keyPair},
		RootCAs:            caCertPool,
		MinVersion:         tls.VersionTLS12,
	}, nil
}

func buildDefaultTLSConfig(cfg *Config) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: isInsecureSkipVerify(cfg), //nolint:gosec
	}
}

func isInsecureSkipVerify(cfg *Config) bool {
	if cfg != nil && cfg.Gateway.HTTPClient != nil {
		return cfg.Gateway.HTTPClient.InsecureTLSVerify
	}
	return false
}

// poolSettings holds the connection pool values the transport is built with.
type poolSettings struct {
	dialTimeout           time.Duration
	keepAlive             time.Duration
	idleConnTimeout       time.Duration
	tlsHandshakeTimeout   time.Duration
	responseHeaderTimeout time.Duration
	maxIdleConns          int
	maxIdleConnsPerHost   int
	maxConnsPerHost       int
}

// newPoolSettings returns the default pool values overridden by the pool section when
// there is one.
//
// The response header timeout has no default: the per-route deadline already bounds the
// wait for the backend headers.
func newPoolSettings(pool *Pool) poolSettings {
	settings := poolSettings{
		dialTimeout:         DefaultTimeout,
		keepAlive:           DefaultKeepAlive,
		idleConnTimeout:     DefaultIdleConnTimeout,
		tlsHandshakeTimeout: DefaultTimeout,
		maxIdleConns:        DefaultConns,
		maxIdleConnsPerHost: DefaultConns,
		maxConnsPerHost:     DefaultConns,
	}
	if pool == nil {
		return settings
	}
	settings.dialTimeout = durationOrDefault(pool.Timeout, settings.dialTimeout)
	settings.keepAlive = durationOrDefault(pool.KeepAlive, settings.keepAlive)
	settings.idleConnTimeout = durationOrDefault(pool.IdleConnTimeout, settings.idleConnTimeout)
	settings.tlsHandshakeTimeout = durationOrDefault(pool.TLSHandshakeTimeout, settings.tlsHandshakeTimeout)
	settings.responseHeaderTimeout = durationOrDefault(pool.ResponseHeaderTimeout, settings.responseHeaderTimeout)
	// The counters are taken as they are: zero keeps its net/http meaning (no limit, or
	// the net/http default for idle connections per host) for a pool built in code.
	settings.maxIdleConns = pool.MaxIdleConns
	settings.maxIdleConnsPerHost = pool.MaxIdleConnsPerHost
	settings.maxConnsPerHost = pool.MaxConnsPerHost
	return settings
}

// durationOrDefault returns the configured duration, or the default when it is not set.
// Validation requires every pool duration, but a pool built in code may leave some out.
func durationOrDefault(value *Duration, defaultValue time.Duration) time.Duration {
	if value == nil {
		return defaultValue
	}
	return value.Duration
}

// buildTransport is the only place where the transport is assembled, so that a client
// setting cannot be honored with a pool section and silently dropped without one.
func buildTransport(clientCfg HTTPClient, tlsConfig *tls.Config) *http.Transport {
	pool := newPoolSettings(clientCfg.Pool)
	transport := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
		DialContext: (&net.Dialer{
			Timeout:   pool.dialTimeout,
			KeepAlive: pool.keepAlive,
		}).DialContext,
		MaxIdleConns:          pool.maxIdleConns,
		MaxIdleConnsPerHost:   pool.maxIdleConnsPerHost,
		MaxConnsPerHost:       pool.maxConnsPerHost,
		IdleConnTimeout:       pool.idleConnTimeout,
		TLSHandshakeTimeout:   pool.tlsHandshakeTimeout,
		ResponseHeaderTimeout: pool.responseHeaderTimeout,
		ExpectContinueTimeout: ContinueDefaultTimeout,
		DisableCompression:    clientCfg.DisableCompression,
	}
	if clientCfg.EnableHTTP2 {
		// HTTP/2 has to be asked for: net/http leaves it off on a transport that brings
		// its own TLS config and dialer. Without it Protocols stays nil, which keeps the
		// transport on HTTP/1. Cleartext HTTP/2 (h2c) is not enabled either way.
		protocols := new(http.Protocols)
		protocols.SetHTTP1(true)
		protocols.SetHTTP2(true)
		transport.Protocols = protocols
	}
	return transport
}

//nolint:bodyclose
func mapRoutesFromConfigToGateway(
	gwConfig Gateway,
	predicateFactory *predicate.Factory,
	filterFactory *filter.Factory,
	logger *slog.Logger) (gateway.Routes, error) {
	out := make(gateway.Routes, 0)
	for _, route := range gwConfig.Routes {
		predicates, err := mapPredicatesFromConfigToGateway(predicateFactory, route.Predicates...)
		if err != nil {
			return nil, fmt.Errorf("map routes from config to gateway failed: %w", err)
		}
		globalFilters, err := mapFiltersFromConfigToGateway(filterFactory, gwConfig.GlobalFilters...)
		if err != nil {
			return nil, fmt.Errorf("map routes from config to gateway failed: %w", err)
		}
		filters, err := mapFiltersFromConfigToGateway(filterFactory, route.Filters...)
		if err != nil {
			return nil, fmt.Errorf("map routes from config to gateway failed: %w", err)
		}
		timeout := calculateTimeout(route.Timeout, gwConfig.GlobalTimeout)
		// The route id is attached here, once per route, instead of on every request:
		// filters and the error handler log through the route logger as it is.
		routeLogger := logger.With(routeLogKey, route.ID)
		circuitBreaker := mapCircuitBreakerFromConfigToGateway(route.ID, route.CircuitBreaker, routeLogger)
		buildRoute, err := gateway.NewRoute(
			route.ID, route.URI, predicates, globalFilters, filters, timeout, circuitBreaker, routeLogger)
		if err != nil {
			return nil, fmt.Errorf("map routes from config to gateway failed: %w", err)
		}
		buildRoute.Streaming = route.Streaming
		out = append(out, *buildRoute)
	}
	return out, nil
}

func calculateTimeout(routeTimeout, globalTimeout Duration) time.Duration {
	if routeTimeout.Duration > 0 {
		return routeTimeout.Duration
	}
	if globalTimeout.Duration > 0 {
		return globalTimeout.Duration
	}
	return DefaultTimeout
}

func mapPredicatesFromConfigToGateway(
	predicateFactory *predicate.Factory,
	predicates ...ParameterizedItem) (gateway.Predicates, error) {
	out := make(gateway.Predicates, 0)
	for _, pred := range predicates {
		gwPred, err := predicateFactory.Build(pred.Name, pred.Args)
		if err != nil {
			return nil, fmt.Errorf("parse predicates failed: %w", err)
		}
		out = append(out, gwPred)
	}
	return out, nil
}

func mapFiltersFromConfigToGateway(
	filterFactory *filter.Factory,
	filters ...ParameterizedItem) (gateway.Filters, error) {
	out := make(gateway.Filters, 0)
	for _, fi := range filters {
		gwFilter, err := filterFactory.Build(fi.Name, fi.Args)
		if err != nil {
			return nil, fmt.Errorf("parse filters failed: %w", err)
		}
		out = append(out, gwFilter)
	}
	return out, nil
}

// isRequestSuccessful is the circuit breaker success policy for gateway routes: any
// error counts as a failure, since backend 5xx responses (ErrInternalServer), network
// errors and timeouts all signal an unhealthy backend. The exception is the client
// cancelling its own request, which says nothing about backend health and would
// otherwise let a burst of client disconnections trip the breaker.
func isRequestSuccessful(err error) bool {
	return err == nil || errors.Is(err, context.Canceled)
}

// logCircuitBreakerStateChange returns the circuit breaker callback that logs every state
// change. Without it a breaker that opens makes the gateway answer 503 and nothing in the
// logs says why. Opening is the only change that takes the route out of service, hence
// the only one logged at warn level.
//
// The breaker runs the callback while it holds its mutex: it must not call back into the
// breaker, and a slow log handler holds up the requests of the route meanwhile.
func logCircuitBreakerStateChange(logger *slog.Logger) func(name string, previous, current circuitbreaker.State) {
	return func(name string, previous, current circuitbreaker.State) {
		level := slog.LevelInfo
		if current == circuitbreaker.StateOpen {
			level = slog.LevelWarn
		}
		// The states are logged by name: a State is an integer, which is what a JSON
		// handler would print for it.
		logger.Log(context.Background(), level, "circuit breaker state changed",
			"breaker", name,
			"from", previous.String(),
			"to", current.String())
	}
}

//nolint:bodyclose
func mapCircuitBreakerFromConfigToGateway(
	name string, circuitBreaker CircuitBreaker, logger *slog.Logger) gateway.CircuitBreaker[*http.Response] {
	if !circuitBreaker.Enabled {
		return nil
	}
	settings := circuitbreaker.Settings{
		Name:        name,
		MaxRequests: uint32(circuitBreaker.NumAllowedHalfOpenCalls), //nolint:gosec
		Interval:    circuitBreaker.Interval.Duration,
		Timeout:     circuitBreaker.WaitDurationInOpenState.Duration,
		ReadyToTrip: circuitbreaker.DefaultReadyToTrip(
			circuitBreaker.MinRequestsThreshold, circuitBreaker.FailureRateThreshold),
		OnStateChange: logCircuitBreakerStateChange(logger),
		IsSuccessful:  isRequestSuccessful,
	}
	return circuitbreaker.NewCircuitBreaker[*http.Response](settings)
}
