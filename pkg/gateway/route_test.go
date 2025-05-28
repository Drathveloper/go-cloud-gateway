package gateway_test

import (
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func TestRoute_CombineGlobalFilters(t *testing.T) {
	tests := []struct {
		name          string
		filters       []gateway.Filter
		globalFilters []gateway.Filter
		expectedOrder []string
	}{
		{
			name: "combine global filters should succeed with expected order when route has filters",
			filters: []gateway.Filter{
				&DummyFilter{"DF1", nil, nil},
				&DummyFilter{"DF2", nil, nil},
			},
			globalFilters: []gateway.Filter{
				&DummyFilter{"DGF1", nil, nil},
				&DummyFilter{"DGF2", nil, nil},
			},
			expectedOrder: []string{"DGF1", "DGF2", "DF1", "DF2"},
		},
		{
			name:    "combine global filters should succeed with expected order when route doesn't have filters",
			filters: nil,
			globalFilters: []gateway.Filter{
				&DummyFilter{"DGF1", nil, nil},
				&DummyFilter{"DGF2", nil, nil},
			},
			expectedOrder: []string{"DGF1", "DGF2"},
		},
		{
			name: "combine global filters should succeed with expected order when no global filters present",
			filters: []gateway.Filter{
				&DummyFilter{"DF1", nil, nil},
				&DummyFilter{"DF2", nil, nil},
			},
			expectedOrder: []string{"DF1", "DF2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := gateway.NewRoute("id", "/test", nil, tt.filters, 0)

			allFilters := route.CombineGlobalFilters(tt.globalFilters...)

			actualOrder := make([]string, 0)
			for _, filter := range allFilters {
				actualOrder = append(actualOrder, filter.Name())
			}

			if !slices.Equal(tt.expectedOrder, actualOrder) {
				t.Errorf("expected order %v actual %v", tt.expectedOrder, actualOrder)
			}
		})
	}
}

func TestRoute_GetDestinationURLStr(t *testing.T) {
	tests := []struct {
		name        string
		routeURL    string
		reqURL      string
		expectedURL string
	}{
		{
			name:        "get destination url should succeed when no query params present",
			routeURL:    "https://example.org",
			reqURL:      "/server/test",
			expectedURL: "https://example.org/server/test",
		},
		{
			name:        "get destination url should succeed when query params present",
			routeURL:    "https://example.org",
			reqURL:      "/server/test?param=value",
			expectedURL: "https://example.org/server/test?param=value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := gateway.NewRoute("someRoute", tt.routeURL, nil, nil, 0)
			reqURL, _ := url.Parse(tt.reqURL)

			actual := route.GetDestinationURLStr(reqURL)

			if tt.expectedURL != actual {
				t.Errorf("expected url %s actual %s", tt.expectedURL, actual)
			}
		})
	}
}

func TestRoutes_FindMatching(t *testing.T) {
	matchedPredicate := DummyPredicate{true}
	unMatchedPredicate := DummyPredicate{false}
	tests := []struct {
		name          string
		routes        []gateway.Route
		expectedRoute string
	}{
		{
			name: "find matching should succeed when one route matched predicate",
			routes: []gateway.Route{
				*gateway.NewRoute("R1", "/test1", []gateway.Predicate{unMatchedPredicate}, nil, 0),
				*gateway.NewRoute("R2", "/test2", []gateway.Predicate{matchedPredicate}, nil, 0),
			},
			expectedRoute: "R2",
		},
		{
			name: "find matching should succeed when no route matched predicate",
			routes: []gateway.Route{
				*gateway.NewRoute("R1", "/test1", []gateway.Predicate{unMatchedPredicate}, nil, 0),
				*gateway.NewRoute("R2", "/test2", []gateway.Predicate{unMatchedPredicate}, nil, 0),
			},
			expectedRoute: "",
		},
		{
			name: "find matching should succeed and match first route when multiple routes matches",
			routes: []gateway.Route{
				*gateway.NewRoute("R1", "/test1", []gateway.Predicate{matchedPredicate}, nil, 0),
				*gateway.NewRoute("R2", "/test2", []gateway.Predicate{matchedPredicate}, nil, 0),
			},
			expectedRoute: "R1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes := gateway.Routes(tt.routes)
			req := &http.Request{}

			route := routes.FindMatching(req)

			if route != nil {
				if route.ID != tt.expectedRoute {
					t.Errorf("expected route with id %s actual %s", tt.expectedRoute, route.ID)
				}
			} else {
				if tt.expectedRoute != "" {
					t.Errorf("actual route is empty but expected route is %s", tt.expectedRoute)
				}
			}
		})
	}
}
