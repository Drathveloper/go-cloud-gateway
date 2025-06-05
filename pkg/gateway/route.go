package gateway

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Route struct {
	ID         string
	URI        string
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
	logger *slog.Logger) *Route {
	return &Route{
		ID:         id,
		URI:        uri,
		Predicates: predicates,
		Filters:    filters,
		Timeout:    timeout,
		Logger:     logger,
	}
}

func (r *Route) CombineGlobalFilters(globalFilters ...Filter) Filters {
	allFilters := globalFilters
	return append(allFilters, r.Filters...)
}

func (r *Route) GetDestinationURL(reqURL *url.URL) string {
	var b strings.Builder

	b.Grow(len(r.URI) + len(reqURL.Path) + len(reqURL.RawQuery) + 1)

	// Unión segura de URI base + path
	switch {
	case strings.HasSuffix(r.URI, "/") && strings.HasPrefix(reqURL.Path, "/"):
		b.WriteString(r.URI[:len(r.URI)-1]) // remove trailing slash
	case !strings.HasSuffix(r.URI, "/") && !strings.HasPrefix(reqURL.Path, "/"):
		b.WriteString(r.URI)
		b.WriteByte('/')
	default:
		b.WriteString(r.URI)
	}
	b.WriteString(reqURL.Path)

	if reqURL.RawQuery != "" {
		b.WriteByte('?')
		b.WriteString(reqURL.RawQuery)
	}

	return b.String()
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
