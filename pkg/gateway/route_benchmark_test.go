package gateway_test

import (
	"net/url"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func BenchmarkGetDestinationURL_Optimized(b *testing.B) {
	r := &gateway.Route{
		URI: &url.URL{
			Scheme: "https",
			Host:   "backend.local",
		},
	}
	u := &url.URL{
		Path:     "/users",
		RawQuery: "id=123&active=true",
	}
	b.ResetTimer()
	for range b.N {
		_ = r.GetDestinationURL(u)
	}
}
