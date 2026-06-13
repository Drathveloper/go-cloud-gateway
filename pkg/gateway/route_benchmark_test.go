package gateway_test

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func BenchmarkRoutesFindMatching(b *testing.B) {
	const numRoutes = 20
	routes := make(gateway.Routes, 0, numRoutes)
	for i := range numRoutes {
		// only the last route matches: worst case scan
		routes = append(routes, gateway.Route{
			ID:         "r" + strconv.Itoa(i),
			Predicates: gateway.Predicates{DummyPredicate{i == numRoutes-1}},
		})
	}
	req := &http.Request{}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if routes.FindMatching(req) == nil {
			b.Fatal("expected a match")
		}
	}
}

func BenchmarkGetDestinationURL_Optimized(b *testing.B) {
	r := &gateway.Route{
		URI: url.URL{
			Scheme: "https",
			Host:   "backend.local",
		},
	}
	u := &url.URL{
		Path:     "/users",
		RawQuery: "id=123&active=true",
	}
	b.ResetTimer()
	for b.Loop() {
		_ = r.GetDestinationURL(u)
	}
}
