package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func NewRoutes(
	cfg *Config,
	predicateFactory *predicate.Factory,
	filterFactory *filter.Factory) (gateway.Routes, error) {
	return mapRoutesFromConfigToGateway(cfg.Gateway, predicateFactory, filterFactory)
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
		return buildConfiguredHTTPClient(cfg, tlsConfig), nil
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
		return nil, fmt.Errorf("failed to load mTLS cert/key pair: %w", err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM([]byte(cfg.Gateway.HTTPClient.MTLS.CA)) {
		return nil, fmt.Errorf("failed to load mTLS CA cert")
	}
	return &tls.Config{
		InsecureSkipVerify: isInsecureSkipVerify(cfg), // nolint:gosec
		Certificates:       []tls.Certificate{keyPair},
		RootCAs:            caCertPool,
		MinVersion:         tls.VersionTLS12,
	}, nil
}

func buildDefaultTLSConfig(cfg *Config) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: isInsecureSkipVerify(cfg), // nolint:gosec
	}
}

func isInsecureSkipVerify(cfg *Config) bool {
	if cfg != nil && cfg.Gateway.HTTPClient != nil {
		return cfg.Gateway.HTTPClient.InsecureTLSVerify
	}
	return false
}

func buildConfiguredHTTPClient(config *Config, tlsConfig *tls.Config) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		DialContext: (&net.Dialer{
			Timeout:   config.Gateway.HTTPClient.Pool.ConnectTimeout.Duration,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        config.Gateway.HTTPClient.Pool.MaxIdleConns,
		MaxIdleConnsPerHost: config.Gateway.HTTPClient.Pool.MaxIdleConnsPerHost,
		MaxConnsPerHost:     config.Gateway.HTTPClient.Pool.MaxConnsPerHost,
		IdleConnTimeout:     config.Gateway.HTTPClient.Pool.IdleConnTimeout.Duration,
		TLSHandshakeTimeout: config.Gateway.HTTPClient.Pool.TLSHandshakeTimeout.Duration,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   config.Gateway.HTTPClient.Pool.ConnectTimeout.Duration,
	}
}

func buildDefaultHTTPClient(tlsConfig *tls.Config) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		DialContext: (&net.Dialer{
			Timeout: DefaultTimeout,
		}).DialContext,
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: 0,
		MaxConnsPerHost:     0,
		IdleConnTimeout:     0,
		TLSHandshakeTimeout: DefaultTimeout,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   DefaultTimeout,
	}
}

func mapRoutesFromConfigToGateway(
	gw Gateway,
	predicateFactory *predicate.Factory,
	filterFactory *filter.Factory) (gateway.Routes, error) {
	out := make(gateway.Routes, 0)
	for _, route := range gw.Routes {
		predicates, err := mapPredicatesFromConfigToGateway(route.Predicates, predicateFactory)
		if err != nil {
			return nil, fmt.Errorf("map routes from config to gateway failed: %w", err)
		}
		filters, err := mapFiltersFromConfigToGateway(route.Filters, filterFactory)
		if err != nil {
			return nil, fmt.Errorf("map routes from config to gateway failed: %w", err)
		}
		timeout := calculateTimeout(route.Timeout, gw.GlobalTimeout)
		out = append(out, *gateway.NewRoute(route.ID, route.URI, predicates, filters, timeout))
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
