package gateway

import (
	"fmt"
	"time"

	"log/slog"
	"net/http"
	"net/url"

	"github.com/drathveloper/go-cloud-gateway/pkg/circuitbreaker"
)

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
type Route struct {
	CircuitBreaker CircuitBreaker[*http.Response]
	URI            *url.URL
	Logger         *slog.Logger
	ID             string
	Predicates     Predicates
	Filters        Filters
	Timeout        time.Duration
}

// NewRoute creates a new route.
func NewRoute(
	routeID string,
	uri string,
	predicates Predicates,
	globalFilters Filters,
	routeFilters Filters,
	timeout time.Duration,
	circuitBreaker CircuitBreaker[*http.Response],
	logger *slog.Logger) (*Route, error) {
	routeURI, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse route uri: %w", err)
	}
	return &Route{
		ID:             routeID,
		URI:            routeURI,
		Predicates:     predicates,
		Filters:        append(globalFilters, routeFilters...),
		Timeout:        timeout,
		CircuitBreaker: circuitBreaker,
		Logger:         logger,
	}, nil
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
func (r Routes) FindMatching(req *http.Request) *Route {
	for _, route := range r {
		if route.Predicates.TestAll(req) {
			return &route
		}
	}
	return nil
}
