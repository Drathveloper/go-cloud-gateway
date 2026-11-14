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

	"golang.org/x/net/http2"

	"github.com/drathveloper/go-cloud-gateway/pkg/circuitbreaker"
	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/httpclient"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

// ErrInitializeMTLS is the error returned when the mTLS initialization failed.
var ErrInitializeMTLS = errors.New("failed to initialize mTLS")

// NewRoutes creates a new gateway route from the given config.
func NewRoutes(
	cfg *Config,
	predicateFactory *predicate.Factory,
	filterFactory *filter.Factory,
	logger *slog.Logger) (gateway.Routes, error) {
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
	transport, err := buildTransport(httpClientConfig(cfg), tlsConfig)
	if err != nil {
		return nil, err
	}
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
func buildTransport(clientCfg HTTPClient, tlsConfig *tls.Config) (*http.Transport, error) {
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
		if err := http2.ConfigureTransport(transport); err != nil {
			return nil, fmt.Errorf("failed to configure http2 transport: %w", err)
		}
	}
	return transport, nil
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
		circuitBreaker := mapCircuitBreakerFromConfigToGateway(route.ID, route.CircuitBreaker)
		buildRoute, err := gateway.NewRoute(
			route.ID, route.URI, predicates, globalFilters, filters, timeout, circuitBreaker, logger)
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

//nolint:bodyclose
func mapCircuitBreakerFromConfigToGateway(
	name string, circuitBreaker CircuitBreaker) gateway.CircuitBreaker[*http.Response] {
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
		OnStateChange: func(_ string, _, _ circuitbreaker.State) {},
		IsSuccessful:  isRequestSuccessful,
	}
	return circuitbreaker.NewCircuitBreaker[*http.Response](settings)
}
