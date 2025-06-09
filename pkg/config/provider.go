package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

var ErrInitializeMTLS = errors.New("failed to initialize mTLS")

func NewRoutes(
	cfg *Config,
	predicateFactory *predicate.Factory,
	filterFactory *filter.Factory,
	logger *slog.Logger) (gateway.Routes, error) {
	return mapRoutesFromConfigToGateway(cfg.Gateway, predicateFactory, filterFactory, logger)
}

func NewGlobalFilters(
	cfg *Config,
	filterFactory *filter.Factory) (gateway.Filters, error) {
	return mapFiltersFromConfigToGateway(cfg.Gateway.GlobalFilters, filterFactory)
}

func NewHTTPClient(cfg *Config) (*http.Client, error) {
	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}
	if cfg != nil && cfg.Gateway.HTTPClient != nil && cfg.Gateway.HTTPClient.Pool != nil {
		return buildConfiguredHTTPClient(cfg, tlsConfig)
	}
	return buildDefaultHTTPClient(tlsConfig), nil
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

func buildConfiguredHTTPClient(config *Config, tlsConfig *tls.Config) (*http.Client, error) {
	transport := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
		DialContext: (&net.Dialer{
			Timeout:   config.Gateway.HTTPClient.Pool.Timeout.Duration,
			KeepAlive: config.Gateway.HTTPClient.Pool.KeepAlive.Duration,
		}).DialContext,
		MaxIdleConns:          config.Gateway.HTTPClient.Pool.MaxIdleConns,
		MaxIdleConnsPerHost:   config.Gateway.HTTPClient.Pool.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.Gateway.HTTPClient.Pool.MaxConnsPerHost,
		IdleConnTimeout:       config.Gateway.HTTPClient.Pool.IdleConnTimeout.Duration,
		TLSHandshakeTimeout:   config.Gateway.HTTPClient.Pool.TLSHandshakeTimeout.Duration,
		ExpectContinueTimeout: ContinueDefaultTimeout,
	}
	if config.Gateway.HTTPClient.EnableHTTP2 {
		if err := http2.ConfigureTransport(transport); err != nil {
			return nil, fmt.Errorf("failed to configure http2 transport: %w", err)
		}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   config.Gateway.HTTPClient.Pool.Timeout.Duration,
	}, nil
}

func buildDefaultHTTPClient(tlsConfig *tls.Config) *http.Client {
	transport := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
		DialContext: (&net.Dialer{
			Timeout:   DefaultTimeout,
			KeepAlive: DefaultKeepAlive,
		}).DialContext,
		MaxIdleConns:          DefaultConns,
		MaxIdleConnsPerHost:   DefaultConns,
		MaxConnsPerHost:       DefaultConns,
		IdleConnTimeout:       DefaultIdleConnTimeout,
		TLSHandshakeTimeout:   DefaultTimeout,
		ExpectContinueTimeout: ContinueDefaultTimeout,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   DefaultTimeout,
	}
}

func mapRoutesFromConfigToGateway(
	gwConfig Gateway,
	predicateFactory *predicate.Factory,
	filterFactory *filter.Factory,
	logger *slog.Logger) (gateway.Routes, error) {
	out := make(gateway.Routes, 0)
	for _, route := range gwConfig.Routes {
		predicates, err := mapPredicatesFromConfigToGateway(route.Predicates, predicateFactory)
		if err != nil {
			return nil, fmt.Errorf("map routes from config to gateway failed: %w", err)
		}
		filters, err := mapFiltersFromConfigToGateway(route.Filters, filterFactory)
		if err != nil {
			return nil, fmt.Errorf("map routes from config to gateway failed: %w", err)
		}
		timeout := calculateTimeout(route.Timeout, gwConfig.GlobalTimeout)
		buildRoute, err := gateway.NewRoute(route.ID, route.URI, predicates, filters, timeout, logger)
		if err != nil {
			return nil, fmt.Errorf("map routes from config to gateway failed: %w", err)
		}
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
	predicates []ParameterizedItem,
	predicateFactory *predicate.Factory) (gateway.Predicates, error) {
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
	filters []ParameterizedItem,
	filterFactory *filter.Factory) (gateway.Filters, error) {
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
