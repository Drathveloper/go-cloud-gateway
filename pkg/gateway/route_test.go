package gateway_test

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func TestNewRoute(t *testing.T) {
	tests := []struct {
		expectedErr error
		name        string
		uri         string
		expectedURI string
		expectedMsg string
	}{
		{
			name:        "new route should succeed when uri is http with host and port",
			uri:         "http://localhost:8080",
			expectedURI: "http://localhost:8080",
		},
		{
			name:        "new route should succeed when uri is https with host",
			uri:         "https://example.org",
			expectedURI: "https://example.org",
		},
		{
			name:        "new route should succeed when uri scheme is upper case",
			uri:         "HTTP://example.org",
			expectedURI: "http://example.org",
		},
		{
			name:        "new route should return error when uri is host and port without scheme",
			uri:         "localhost:8080",
			expectedErr: gateway.ErrInvalidRouteURI,
			expectedMsg: "invalid route uri \"localhost:8080\": expected an http or https url with a host, such as http://localhost:8080",
		},
		{
			name:        "new route should return error when uri is a bare host name",
			uri:         "backend",
			expectedErr: gateway.ErrInvalidRouteURI,
			expectedMsg: "invalid route uri \"backend\": expected an http or https url with a host, such as http://localhost:8080",
		},
		{
			name:        "new route should return error when uri is only a path",
			uri:         "/just/path",
			expectedErr: gateway.ErrInvalidRouteURI,
			expectedMsg: "invalid route uri \"/just/path\": expected an http or https url with a host, such as http://localhost:8080",
		},
		{
			name:        "new route should return error when uri scheme is not http or https",
			uri:         "ftp://x",
			expectedErr: gateway.ErrInvalidRouteURI,
			expectedMsg: "invalid route uri \"ftp://x\": expected an http or https url with a host, such as http://localhost:8080",
		},
		{
			name:        "new route should return error when uri has no host",
			uri:         "http://",
			expectedErr: gateway.ErrInvalidRouteURI,
			expectedMsg: "invalid route uri \"http://\": expected an http or https url with a host, such as http://localhost:8080",
		},
		{
			name:        "new route should return error when uri has a path but no host",
			uri:         "https:///api",
			expectedErr: gateway.ErrInvalidRouteURI,
			expectedMsg: "invalid route uri \"https:///api\": expected an http or https url with a host, such as http://localhost:8080",
		},
		{
			name:        "new route should return error when uri is empty",
			uri:         "",
			expectedErr: gateway.ErrInvalidRouteURI,
			expectedMsg: "invalid route uri \"\": expected an http or https url with a host, such as http://localhost:8080",
		},
		{
			name:        "new route should return error when uri cannot be parsed",
			uri:         "http://exa mple.org",
			expectedErr: gateway.ErrInvalidRouteURI,
			expectedMsg: "invalid route uri: parse \"http://exa mple.org\": invalid character \" \" in host name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, err := gateway.NewRoute("someRoute", tt.uri, nil, nil, nil, 0, nil, nil)

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected err %v actual %v", tt.expectedErr, err)
			}
			if tt.expectedErr != nil {
				if tt.expectedMsg != err.Error() {
					t.Errorf("expected err message %s actual %s", tt.expectedMsg, err)
				}
				if route != nil {
					t.Errorf("expected nil route actual %v", route)
				}
				return
			}
			if route == nil {
				t.Fatal("expected a route actual nil")
			}
			if tt.expectedURI != route.URI.String() {
				t.Errorf("expected uri %s actual %s", tt.expectedURI, route.URI.String())
			}
		})
	}
}

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

func TestRoutes_FindMatching_ReturnsACopy(t *testing.T) {
	routes := gateway.Routes{
		{
			ID:         "r1",
			URI:        url.URL{Scheme: "https", Host: "backend.internal"},
			Timeout:    time.Minute,
			Predicates: gateway.Predicates{DummyPredicate{true}},
		},
	}

	matched := routes.FindMatching(&http.Request{})
	if matched == nil {
		t.Fatal("expected a match")
	}
	// A filter mutating the matched route by mistake must corrupt its own copy,
	// never the shared route table.
	matched.ID = "mutated"
	matched.Timeout = 0
	matched.URI.Host = "evil.example.org"

	if routes[0].ID != "r1" || routes[0].Timeout != time.Minute {
		t.Errorf("expected the route table to be untouched, actual %+v", routes[0])
	}
	if routes[0].URI.Host != "backend.internal" {
		t.Errorf("expected the route URI to be untouched, actual %s", routes[0].URI.Host)
	}
}

func TestRoutes_FindMatching(t *testing.T) {
	matchedPredicate := DummyPredicate{true}
	unMatchedPredicate := DummyPredicate{false}

	unmatchedRoute1, _ := gateway.NewRoute("R1", "http://localhost:8080", []gateway.Predicate{unMatchedPredicate}, nil, nil, 0, nil, nil)
	matchedRoute1, _ := gateway.NewRoute("R1", "http://localhost:8080", []gateway.Predicate{matchedPredicate}, nil, nil, 0, nil, nil)
	unmatchedRoute2, _ := gateway.NewRoute("R2", "http://localhost:8080", []gateway.Predicate{unMatchedPredicate}, nil, nil, 0, nil, nil)
	matchedRoute2, _ := gateway.NewRoute("R2", "http://localhost:8080", []gateway.Predicate{matchedPredicate}, nil, nil, 0, nil, nil)

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
