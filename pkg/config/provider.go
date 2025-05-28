package config

import (
	"fmt"
	"gateway/pkg/filter"
	"gateway/pkg/gateway"
	"gateway/pkg/predicate"
	"net"
	"net/http"
	"time"
)

func NewRoutes(
	cfg *Config,
	predicateFactory *predicate.Factory,
	filterFactory *filter.Factory) (gateway.Routes, error) {
	return mapRoutesFromConfigToGateway(cfg.Gateway.Routes, predicateFactory, filterFactory)
}

func NewGlobalFilters(
	cfg *Config,
	filterFactory *filter.Factory) (gateway.Filters, error) {
	return mapFiltersFromConfigToGateway(cfg.Gateway.GlobalFilters, filterFactory)
}

func NewHTTPClient(cfg *Config) (*http.Client, error) {
	var transport *http.Transport
	var timeout time.Duration
	if cfg.Gateway.HTTPClient != nil {
		timeout = cfg.Gateway.HTTPClient.ConnectTimeout.Duration
		transport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   cfg.Gateway.HTTPClient.ConnectTimeout.Duration,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        cfg.Gateway.HTTPClient.MaxIdleConns,
			MaxIdleConnsPerHost: cfg.Gateway.HTTPClient.MaxIdleConnsPerHost,
			MaxConnsPerHost:     cfg.Gateway.HTTPClient.MaxConnsPerHost,
			IdleConnTimeout:     cfg.Gateway.HTTPClient.IdleConnTimeout.Duration,
			TLSHandshakeTimeout: cfg.Gateway.HTTPClient.TLSHandshakeTimeout.Duration,
		}
	} else {
		timeout = cfg.Gateway.HTTPClient.ConnectTimeout.Duration
		transport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        0,
			MaxIdleConnsPerHost: 0,
			MaxConnsPerHost:     0,
			IdleConnTimeout:     0,
			TLSHandshakeTimeout: 10 * time.Second,
		}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

func NewGlobalTimeout(cfg *Config) time.Duration {
	if cfg.GlobalTimeout.Duration != 0 {
		return cfg.GlobalTimeout.Duration
	}
	return 10 * time.Second
}

func mapRoutesFromConfigToGateway(
	routes []Route,
	predicateFactory *predicate.Factory,
	filterFactory *filter.Factory) (gateway.Routes, error) {
	out := make(gateway.Routes, 0)
	for _, route := range routes {
		predicates, err := mapPredicatesFromConfigToGateway(route.Predicates, predicateFactory)
		if err != nil {
			return nil, fmt.Errorf("map routes from config to gateway failed: %w", err)
		}
		filters, err := mapFiltersFromConfigToGateway(route.Filters, filterFactory)
		if err != nil {
			return nil, fmt.Errorf("map routes from config to gateway failed: %w", err)
		}
		out = append(out, *gateway.NewRoute(route.ID, route.URI, predicates, filters, route.Timeout))
	}
	return out, nil
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
