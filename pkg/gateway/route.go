package gateway

import (
	"net/http"
	"net/url"
	"time"
)

type Route struct {
	ID         string
	URI        string
	Predicates Predicates
	Filters    Filters
	Timeout    time.Duration
}

func NewRoute(
	id,
	uri string,
	predicates Predicates,
	filters Filters,
	timeout time.Duration) *Route {
	return &Route{
		ID:         id,
		URI:        uri,
		Predicates: predicates,
		Filters:    filters,
		Timeout:    timeout,
	}
}

func (r *Route) CombineGlobalFilters(globalFilters ...Filter) Filters {
	allFilters := globalFilters
	return append(allFilters, r.Filters...)
}

func (r *Route) GetDestinationURLStr(reqURL *url.URL) string {
	destURL := r.URI + reqURL.Path
	if reqURL.RawQuery != "" {
		destURL += "?" + reqURL.RawQuery
	}
	return destURL
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
