package bootstrap_test

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
)

// answeredByHeader is the response header a route backend tells its route id with. A
// header and not the body, so that HEAD requests are told apart as well.
const answeredByHeader = "X-Answered-By"

// predicateRequest is a request sent to a gateway and the route expected to take it.
type predicateRequest struct {
	headers http.Header
	name    string
	// method defaults to GET.
	method string
	// target is the path and the query string, sent as written. It defaults to "/".
	target string
	// host is the Host header. It defaults to the address of the gateway.
	host string
	// route is the id of the route whose backend must answer, empty when no route must
	// match and the gateway must answer 404.
	route string
}

// startRoutedGateway starts a gateway built from the given YAML configuration in which
// every route has a backend of its own that answers with the route id. The uri of each
// route in the YAML is only there to make it a valid configuration: it is replaced by the
// URL of the backend of the route once the configuration has been read.
func startRoutedGateway(t *testing.T, cfgYAML string) *httptest.Server {
	t.Helper()
	cfg := readYAMLConfig(t, cfgYAML)
	for i := range cfg.Gateway.Routes {
		routeID := cfg.Gateway.Routes[i].ID
		backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set(answeredByHeader, routeID)
		}))
		t.Cleanup(backend.Close)
		cfg.Gateway.Routes[i].URI = backend.URL
	}
	gatewayServer := newEndToEndGateway(t, bootstrap.NewOptionsBuilder(cfg).Build())
	gatewayServer.Start()
	return gatewayServer
}

// sendPredicateRequest sends the request to the gateway and returns the status of the
// response and the id of the route whose backend answered, empty when none did.
func sendPredicateRequest(t *testing.T, gatewayServer *httptest.Server, sent predicateRequest) (int, string) {
	t.Helper()
	method, target := sent.method, sent.target
	if method == "" {
		method = http.MethodGet
	}
	if target == "" {
		target = "/"
	}
	request, err := http.NewRequestWithContext(t.Context(), method, gatewayServer.URL+target, nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	if sent.host != "" {
		request.Host = sent.host
	}
	// The several values of a header are sent as separate header lines.
	maps.Copy(request.Header, sent.headers)
	res := doEndToEndRequest(t, gatewayServer.Client(), request)
	return res.status, res.header.Get(answeredByHeader)
}

const pathPredicateYAML = `
gateway:
  routes:
    - id: users
      uri: http://localhost:8081
      predicates:
        - name: Path
          args:
            patterns:
              - /api/**/users/*
    - id: assets
      uri: http://localhost:8082
      predicates:
        - name: Path
          args:
            patterns:
              - /static/*.css
              - /theme-?.css
    - id: api
      uri: http://localhost:8083
      predicates:
        - name: Path
          args:
            patterns:
              - /api/**
`

const hostPredicateYAML = `
gateway:
  routes:
    - id: admin-port
      uri: http://localhost:8081
      predicates:
        - name: Host
          args:
            patterns:
              - admin.example.org:9000
    - id: apex
      uri: http://localhost:8082
      predicates:
        - name: Host
          args:
            patterns:
              - example.org
              - WWW.Example.ORG
    - id: numbered
      uri: http://localhost:8083
      predicates:
        - name: Host
          args:
            patterns:
              - "app?.example.org"
    - id: subdomain
      uri: http://localhost:8084
      predicates:
        - name: Host
          args:
            patterns:
              - "*.example.org"
    - id: nested
      uri: http://localhost:8085
      predicates:
        - name: Host
          args:
            patterns:
              - "**.example.com"
`

const methodPredicateYAML = `
gateway:
  routes:
    - id: readers
      uri: http://localhost:8081
      predicates:
        - name: Method
          args:
            methods:
              - get
              - Head
    - id: writers
      uri: http://localhost:8082
      predicates:
        - name: Method
          args:
            methods:
              - POST
              - put
`

const headerPredicateYAML = `
gateway:
  routes:
    - id: versioned
      uri: http://localhost:8081
      predicates:
        - name: Header
          args:
            name: X-Api-Version
            regexp: ^v[0-9]+$
    - id: canary
      uri: http://localhost:8082
      predicates:
        - name: Header
          args:
            name: X-Release
            regexp: canary
    - id: untraced
      uri: http://localhost:8083
      predicates:
        - name: Header
          args:
            name: X-Trace
            regexp: ^$
    - id: beta
      uri: http://localhost:8084
      predicates:
        - name: Header
          args:
            name: x-beta
`

const queryPredicateYAML = `
gateway:
  routes:
    - id: versioned
      uri: http://localhost:8081
      predicates:
        - name: Query
          args:
            name: version
            regexp: ^v[0-9]+$
    - id: canary
      uri: http://localhost:8082
      predicates:
        - name: Query
          args:
            name: release
            regexp: canary
    - id: anonymous
      uri: http://localhost:8083
      predicates:
        - name: Query
          args:
            name: token
            regexp: ^$
    - id: flagged
      uri: http://localhost:8084
      predicates:
        - name: Query
          args:
            name: flag
`

const cookiePredicateYAML = `
gateway:
  routes:
    - id: versioned
      uri: http://localhost:8081
      predicates:
        - name: Cookie
          args:
            name: version
            regexp: ^v[0-9]+$
    - id: canary
      uri: http://localhost:8082
      predicates:
        - name: Cookie
          args:
            name: release
            regexp: canary
    - id: anonymous
      uri: http://localhost:8083
      predicates:
        - name: Cookie
          args:
            name: token
            regexp: ^$
    - id: session
      uri: http://localhost:8084
      predicates:
        - name: Cookie
          args:
            name: session
`

// combinedPredicatesYAML has a route with one predicate of each kind and a second route
// that takes what the first one leaves.
const combinedPredicatesYAML = `
gateway:
  routes:
    - id: orders-beta
      uri: http://localhost:8081
      predicates:
        - name: Path
          args:
            patterns:
              - /orders/**
        - name: Host
          args:
            patterns:
              - "*.example.org"
        - name: Method
          args:
            methods:
              - post
        - name: Header
          args:
            name: X-Beta
        - name: Query
          args:
            name: version
            regexp: ^v[0-9]+$
        - name: Cookie
          args:
            name: session
    - id: orders
      uri: http://localhost:8082
      predicates:
        - name: Path
          args:
            patterns:
              - /orders/**
`

func pathPredicateRequests() []predicateRequest {
	return []predicateRequest{
		{name: "recursive wildcard in the middle should match no segment", target: "/api/users/7", route: "users"},
		{name: "recursive wildcard in the middle should match one segment", target: "/api/v1/users/7", route: "users"},
		{
			name:   "recursive wildcard in the middle should match several segments",
			target: "/api/v1/tenants/users/7",
			route:  "users",
		},
		{
			name:   "recursive wildcard should backtrack past a first candidate that does not fit",
			target: "/api/users/list/users/7",
			route:  "users",
		},
		{
			name:   "recursive wildcard should backtrack past several candidates that do not fit",
			target: "/api/users/a/users/b/c/users/7",
			route:  "users",
		},
		{name: "trailing slash should not be significant", target: "/api/v1/users/7/", route: "users"},
		{name: "query string should not be part of the path", target: "/api/v1/users/7?expand=orders", route: "users"},
		{
			name:   "path without the segment after the literal should fall to the next route",
			target: "/api/v1/users",
			route:  "api",
		},
		{
			name:   "path with a segment after the last wildcard should fall to the next route",
			target: "/api/v1/users/7/orders",
			route:  "api",
		},
		{name: "trailing recursive wildcard should match no segment", target: "/api", route: "api"},
		{name: "trailing recursive wildcard should match several segments", target: "/api/v1/orders/7", route: "api"},
		{name: "wildcard should match within a segment", target: "/static/site.css", route: "assets"},
		{name: "wildcard should not match across segments", target: "/static/vendor/site.css", route: ""},
		{name: "wildcard should not match another suffix", target: "/static/site.js", route: ""},
		{name: "second pattern of the list should be enough", target: "/theme-a.css", route: "assets"},
		{name: "question mark should not match two characters", target: "/theme-ab.css", route: ""},
		{name: "question mark should not match no character", target: "/theme-.css", route: ""},
		{name: "literal segment should not match a longer one", target: "/apis/users/7", route: ""},
		{name: "root should not match", target: "/", route: ""},
	}
}

func hostPredicateRequests() []predicateRequest {
	return []predicateRequest{
		{name: "literal pattern should match the host", host: "example.org", route: "apex"},
		{name: "pattern without port should match the host on any port", host: "example.org:8080", route: "apex"},
		{name: "host should match in any case", host: "EXAMPLE.Org", route: "apex"},
		{name: "pattern should match in any case", host: "www.example.org", route: "apex"},
		{name: "pattern with port should match the host on that port", host: "admin.example.org:9000", route: "admin-port"},
		{
			name:  "pattern with port should not match the host on another port",
			host:  "admin.example.org:9001",
			route: "subdomain",
		},
		{name: "pattern with port should not match the host without port", host: "admin.example.org", route: "subdomain"},
		{name: "question mark should match one character", host: "app1.example.org", route: "numbered"},
		{name: "question mark should match one character on any port", host: "app2.example.org:8443", route: "numbered"},
		{name: "question mark should not match two characters", host: "app12.example.org", route: "subdomain"},
		{name: "question mark should not match no character", host: "app.example.org", route: "subdomain"},
		{name: "wildcard should match one label", host: "shop.example.org", route: "subdomain"},
		{name: "wildcard should not match across labels", host: "eu.shop.example.org", route: ""},
		{name: "recursive wildcard should match one label", host: "shop.example.com", route: "nested"},
		{name: "recursive wildcard should match across labels", host: "eu.shop.example.com:8080", route: "nested"},
		{name: "recursive wildcard should not match no label", host: "example.com", route: ""},
		{name: "literal pattern should not match a longer host", host: "example.org.evil.test", route: ""},
		{name: "host of the gateway should not match", route: ""},
	}
}

func methodPredicateRequests() []predicateRequest {
	return []predicateRequest{
		{name: "method in lower case in the config should match", method: http.MethodGet, route: "readers"},
		{name: "method in mixed case in the config should match", method: http.MethodHead, route: "readers"},
		{name: "method in upper case in the config should match", method: http.MethodPost, route: "writers"},
		{name: "second method of the list in lower case should match", method: http.MethodPut, route: "writers"},
		{name: "method out of every list should not match", method: http.MethodDelete, route: ""},
		{name: "method out of every list should not match by prefix", method: http.MethodPatch, route: ""},
	}
}

func headerPredicateRequests() []predicateRequest {
	return []predicateRequest{
		{name: "header should match on presence", headers: http.Header{"X-Beta": {"true"}}, route: "beta"},
		{name: "header with an empty value should be present", headers: http.Header{"X-Beta": {""}}, route: "beta"},
		{name: "request without the header should not match", route: ""},
		{name: "another header should not match", headers: http.Header{"X-Beta-Tester": {"true"}}, route: ""},
		{name: "value that matches the regexp should match", headers: http.Header{"X-Api-Version": {"v2"}}, route: "versioned"},
		{
			name:    "any value that matches the regexp should be enough",
			headers: http.Header{"X-Api-Version": {"beta", "v3"}},
			route:   "versioned",
		},
		{name: "value that does not match the regexp should not match", headers: http.Header{"X-Api-Version": {"2"}}, route: ""},
		{
			name:    "anchored regexp should not match within the value",
			headers: http.Header{"X-Api-Version": {"rev2"}},
			route:   "",
		},
		{name: "empty value should not match a regexp that needs one", headers: http.Header{"X-Api-Version": {""}}, route: ""},
		{
			name:    "regexp without anchors should match within the value",
			headers: http.Header{"X-Release": {"2026-canary-3"}},
			route:   "canary",
		},
		{name: "regexp without anchors should not match another value", headers: http.Header{"X-Release": {"stable"}}, route: ""},
		{name: "empty value should match a regexp that accepts it", headers: http.Header{"X-Trace": {""}}, route: "untraced"},
		{name: "value should not match a regexp for the empty one", headers: http.Header{"X-Trace": {"abc"}}, route: ""},
	}
}

func queryPredicateRequests() []predicateRequest {
	return []predicateRequest{
		{name: "parameter without value nor equals sign should be present", target: "/?flag", route: "flagged"},
		{name: "parameter with an empty value should be present", target: "/?flag=", route: "flagged"},
		{name: "parameter with a value should be present", target: "/?flag=on", route: "flagged"},
		{name: "parameter after others should be present", target: "/search?q=go&flag", route: "flagged"},
		{name: "parameter with an escaped name should be present", target: "/?fl%61g", route: "flagged"},
		{name: "request without query string should not match", target: "/", route: ""},
		{name: "longer parameter name should not match", target: "/?flags=on", route: ""},
		{name: "parameter name as a value should not match", target: "/?q=flag", route: ""},
		{name: "value that matches the regexp should match", target: "/?version=v2", route: "versioned"},
		{name: "any value that matches the regexp should be enough", target: "/?version=beta&version=v3", route: "versioned"},
		{name: "escaped value should be matched unescaped", target: "/?version=v%32", route: "versioned"},
		{name: "value that does not match the regexp should not match", target: "/?version=2", route: ""},
		{name: "anchored regexp should not match within the value", target: "/?version=rev2", route: ""},
		{name: "empty value should not match a regexp that needs one", target: "/?version=", route: ""},
		{name: "regexp without anchors should match within the value", target: "/?release=2026-canary-3", route: "canary"},
		{name: "regexp without anchors should not match another value", target: "/?release=stable", route: ""},
		{name: "empty value should match a regexp that accepts it", target: "/?token=", route: "anonymous"},
		{name: "missing value should match a regexp that accepts the empty one", target: "/?token", route: "anonymous"},
		{name: "value should not match a regexp for the empty one", target: "/?token=abc", route: ""},
	}
}

func cookiePredicateRequests() []predicateRequest {
	cookies := func(lines ...string) http.Header {
		return http.Header{"Cookie": lines}
	}
	return []predicateRequest{
		{name: "cookie should match on presence", headers: cookies("session=abc"), route: "session"},
		{name: "cookie with an empty value should be present", headers: cookies("session="), route: "session"},
		{name: "cookie after others should be present", headers: cookies("theme=dark; session=abc"), route: "session"},
		{name: "cookie in a second header line should be present", headers: cookies("theme=dark", "session=abc"), route: "session"},
		{name: "request without cookies should not match", route: ""},
		{name: "cookie with a longer name should not match", headers: cookies("xsession=abc"), route: ""},
		{name: "cookie name as a value should not match", headers: cookies("theme=session"), route: ""},
		{name: "value that matches the regexp should match", headers: cookies("version=v2"), route: "versioned"},
		{
			name:    "any value that matches the regexp should be enough",
			headers: cookies("version=beta; version=v3"),
			route:   "versioned",
		},
		{
			name:    "any value in any header line that matches the regexp should be enough",
			headers: cookies("version=beta", "version=v3"),
			route:   "versioned",
		},
		{name: "value that does not match the regexp should not match", headers: cookies("version=2"), route: ""},
		{name: "anchored regexp should not match within the value", headers: cookies("version=rev2"), route: ""},
		{name: "empty value should not match a regexp that needs one", headers: cookies("version="), route: ""},
		{name: "regexp without anchors should match within the value", headers: cookies("release=2026-canary-3"), route: "canary"},
		{name: "regexp without anchors should not match another value", headers: cookies("release=stable"), route: ""},
		{name: "empty value should match a regexp that accepts it", headers: cookies("token="), route: "anonymous"},
		{name: "value should not match a regexp for the empty one", headers: cookies("token=abc"), route: ""},
	}
}

func combinedPredicateRequests() []predicateRequest {
	// matching returns a request that meets every predicate of the first route, changed
	// by the given function.
	matching := func(name, route string, change func(request *predicateRequest)) predicateRequest {
		request := predicateRequest{
			name:    name,
			method:  http.MethodPost,
			target:  "/orders/7?version=v2",
			host:    "shop.example.org",
			headers: http.Header{"X-Beta": {""}, "Cookie": {"session=abc"}},
			route:   route,
		}
		change(&request)
		return request
	}
	return []predicateRequest{
		matching("request that meets every predicate should match", "orders-beta", func(*predicateRequest) {}),
		matching("request that fails the path should match no route", "", func(request *predicateRequest) {
			request.target = "/invoices/7?version=v2"
		}),
		matching("request that fails the host should fall to the next route", "orders", func(request *predicateRequest) {
			request.host = "shop.example.net"
		}),
		matching("request that fails the method should fall to the next route", "orders", func(request *predicateRequest) {
			request.method = http.MethodGet
		}),
		matching("request that fails the header should fall to the next route", "orders", func(request *predicateRequest) {
			request.headers.Del("X-Beta")
		}),
		matching("request that fails the query should fall to the next route", "orders", func(request *predicateRequest) {
			request.target = "/orders/7?version=latest"
		}),
		matching("request that fails the cookie should fall to the next route", "orders", func(request *predicateRequest) {
			request.headers.Del("Cookie")
		}),
	}
}

// The predicates are exercised through a running gateway: the routes come from a YAML
// configuration read by the reader, the requests travel over HTTP and what is asserted is
// which route's backend answered, so the parsing of the arguments, the request as
// net/http hands it to the predicates and the order of the routes are all covered.
func TestEndToEnd_Predicates(t *testing.T) {
	tests := []struct {
		name     string
		cfgYAML  string
		requests []predicateRequest
	}{
		{name: "Path", cfgYAML: pathPredicateYAML, requests: pathPredicateRequests()},
		{name: "Host", cfgYAML: hostPredicateYAML, requests: hostPredicateRequests()},
		{name: "Method", cfgYAML: methodPredicateYAML, requests: methodPredicateRequests()},
		{name: "Header", cfgYAML: headerPredicateYAML, requests: headerPredicateRequests()},
		{name: "Query", cfgYAML: queryPredicateYAML, requests: queryPredicateRequests()},
		{name: "Cookie", cfgYAML: cookiePredicateYAML, requests: cookiePredicateRequests()},
		{name: "every predicate of a route", cfgYAML: combinedPredicatesYAML, requests: combinedPredicateRequests()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gatewayServer := startRoutedGateway(t, tt.cfgYAML)
			for _, sent := range tt.requests {
				t.Run(sent.name, func(t *testing.T) {
					expectedStatus := http.StatusOK
					if sent.route == "" {
						expectedStatus = http.StatusNotFound
					}

					status, route := sendPredicateRequest(t, gatewayServer, sent)

					if status != expectedStatus || route != sent.route {
						t.Errorf("expected status %d from route %q, actual status %d from route %q",
							expectedStatus, sent.route, status, route)
					}
				})
			}
		})
	}
}
