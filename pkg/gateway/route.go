package gateway

import (
	"errors"
	"fmt"
	"time"

	"log/slog"
	"net/http"
	"net/url"
	"slices"

	"github.com/drathveloper/go-cloud-gateway/pkg/circuitbreaker"
)

// ErrInvalidRouteURI is returned when the route uri is not an http or https url with a
// host, or when it carries more than that.
var ErrInvalidRouteURI = errors.New("invalid route uri")

// CircuitBreaker is a circuit breaker.
//
// T is the type of the result of the circuit breaker.
type CircuitBreaker[T any] interface {
	Name() string
	State() circuitbreaker.State
	Counts() circuitbreaker.Counts
	Execute(req func() (T, error)) (T, error)
}

// Route represents a gateway route.
//
// URI is a value on purpose: the shallow copy FindMatching hands to each request
// covers it, so a filter mutating it cannot corrupt the shared route table.
type Route struct {
	CircuitBreaker CircuitBreaker[*http.Response]
	URI            url.URL
	Logger         *slog.Logger
	ID             string
	Predicates     Predicates
	Filters        Filters
	Timeout        time.Duration
	// Streaming marks a route whose responses are long-lived streams (server-sent
	// events, chunked downloads). Timeout then only bounds the time until the response
	// is ready to be sent, and from then on each individual write to the client and
	// each individual read of what is left of the request body, instead of the whole
	// exchange.
	Streaming bool
}

// NewRoute creates a new route.
//
// The uri must be an http or https url with a host and nothing else, such as
// http://localhost:8080: a path, query, fragment or user info would not be used.
// Anything else returns an error wrapping ErrInvalidRouteURI.
//
// The route filters are the global filters followed by the route filters. They are
// copied, so the same globalFilters slice can be passed to several routes.
func NewRoute(
	routeID string,
	uri string,
	predicates Predicates,
	globalFilters Filters,
	routeFilters Filters,
	timeout time.Duration,
	circuitBreaker CircuitBreaker[*http.Response],
	logger *slog.Logger) (*Route, error) {
	routeURI, err := parseRouteURI(uri)
	if err != nil {
		return nil, err
	}
	// Not append(globalFilters, routeFilters...): when globalFilters has spare capacity
	// that writes the route filters into its backing array, and every route built from
	// the same slice would end up with the filters of the last one.
	filters := slices.Concat(globalFilters, routeFilters)
	return &Route{
		ID:             routeID,
		URI:            *routeURI,
		Predicates:     predicates,
		Filters:        filters,
		Timeout:        timeout,
		CircuitBreaker: circuitBreaker,
		Logger:         logger,
	}, nil
}

// parseRouteURI parses the uri of a route and rejects the ones a route cannot work with.
func parseRouteURI(uri string) (*url.URL, error) {
	routeURI, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidRouteURI, err)
	}
	// url.Parse also accepts host:port, a bare name or a path. Those build a route that
	// can never reach a backend, so they are rejected here instead of failing every
	// request once the gateway is running.
	if (routeURI.Scheme != "http" && routeURI.Scheme != "https") || routeURI.Host == "" {
		return nil, fmt.Errorf(
			"%w %q: expected an http or https url with a host, such as http://localhost:8080", ErrInvalidRouteURI, uri)
	}
	// Only the scheme and the host reach the backend: the path and the query are the ones
	// of the request. Anything else written in the uri would be dropped without notice,
	// and a route that looks like it adds a prefix or sends credentials would not.
	if hasMoreThanSchemeAndHost(routeURI) {
		return nil, fmt.Errorf(
			"%w %q: only the scheme and the host are used, so a path, query, fragment or user info is not allowed; "+
				"use a RewritePath filter to change the path", ErrInvalidRouteURI, uri)
	}
	return routeURI, nil
}

// hasMoreThanSchemeAndHost reports whether the uri carries anything GetDestinationURL does
// not use. A lone trailing slash is not counted: it is the same backend.
func hasMoreThanSchemeAndHost(uri *url.URL) bool {
	hasPath := uri.Path != "" && uri.Path != "/"
	return hasPath || uri.RawQuery != "" || uri.Fragment != "" || uri.User != nil
}

// GetDestinationURL returns the destination url for the given request url combining scheme and host from the route
// uri and the rest of elements from the request url.
//
// For example, if the route uri is http://example.org:8080 and the request url is http://localhost:8080/api/v1/users,
// the destination url is http://example.org:8080/api/v1/users.
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

// Routes represent a list of routes.
type Routes []Route

// FindMatching finds the first matching route for the given request.
//
// If no matching route is found, nil is returned.
//
// If multiple matching routes are found, the first one is returned.
//
// The order of the routes in the list is important.
//
// The returned route is a shallow copy: a filter that mutates its value fields
// (ID, Timeout, URI) by mistake corrupts only its own request, never the shared
// route table. Pointer fields (Logger, CircuitBreaker, the predicate and filter
// slices) are still shared across all requests and must be treated as read-only.
func (r Routes) FindMatching(req *http.Request) *Route {
	for i := range r {
		if r[i].Predicates.TestAll(req) {
			// only the matched route is copied, not every scanned candidate
			route := r[i]
			return new(route)
		}
	}
	return nil
}
