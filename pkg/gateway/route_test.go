package gateway_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func TestRoute_GetDestinationURL(t *testing.T) {
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
			route, _ := gateway.NewRoute("someRoute", tt.routeURL, nil, nil, nil, 0, nil, nil)

			reqURL, _ := url.Parse(tt.reqURL)

			actual := route.GetDestinationURL(reqURL)

			if tt.expectedURL != actual.String() {
				t.Errorf("expected url %s actual %s", tt.expectedURL, actual)
			}
		})
	}
}

func TestRoutes_FindMatching(t *testing.T) {
	matchedPredicate := DummyPredicate{true}
	unMatchedPredicate := DummyPredicate{false}

	unmatchedRoute1, _ := gateway.NewRoute("R1", "/test1", []gateway.Predicate{unMatchedPredicate}, nil, nil, 0, nil, nil)
	matchedRoute1, _ := gateway.NewRoute("R1", "/test1", []gateway.Predicate{matchedPredicate}, nil, nil, 0, nil, nil)
	unmatchedRoute2, _ := gateway.NewRoute("R2", "/test1", []gateway.Predicate{unMatchedPredicate}, nil, nil, 0, nil, nil)
	matchedRoute2, _ := gateway.NewRoute("R2", "/test1", []gateway.Predicate{matchedPredicate}, nil, nil, 0, nil, nil)

	tests := []struct {
		name          string
		expectedRoute string
		routes        []gateway.Route
	}{
		{
			name: "find matching should succeed when one route matched predicate",
			routes: []gateway.Route{
				*unmatchedRoute1,
				*matchedRoute2,
			},
			expectedRoute: "R2",
		},
		{
			name: "find matching should succeed when no route matched predicate",
			routes: []gateway.Route{
				*unmatchedRoute1,
				*unmatchedRoute2,
			},
			expectedRoute: "",
		},
		{
			name: "find matching should succeed and match first route when multiple routes matches",
			routes: []gateway.Route{
				*matchedRoute1,
				*matchedRoute2,
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
