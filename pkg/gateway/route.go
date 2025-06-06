package gateway

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

type Route struct {
	ID         string
	URI        *url.URL
	Predicates Predicates
	Filters    Filters
	Timeout    time.Duration
	Logger     *slog.Logger
}

func NewRoute(
	id,
	uri string,
	predicates Predicates,
	filters Filters,
	timeout time.Duration,
	logger *slog.Logger) (*Route, error) {
	routeURI, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse route uri: %w", err)
	}
	return &Route{
		ID:         id,
		URI:        routeURI,
		Predicates: predicates,
		Filters:    filters,
		Timeout:    timeout,
		Logger:     logger,
	}, nil
}

func (r *Route) CombineGlobalFilters(globalFilters ...Filter) Filters {
	allFilters := globalFilters
	return append(allFilters, r.Filters...)
}

func (r *Route) GetDestinationURL(reqURL *url.URL) *url.URL {
	newURL := &url.URL{
		Scheme:   r.URI.Scheme,
		Host:     r.URI.Host,
		Path:     reqURL.Path,
		RawPath:  reqURL.RawPath,
		RawQuery: reqURL.RawQuery,
	}
	return newURL
}

type Routes []Route

func (r Routes) FindMatching(req *http.Request) *Route {
	for _, route := range r {
		if route.Predicates.TestAll(req) {
			return &route
		}
	}
	return nil
}
