package gateway_test

import (
	"net/url"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func BenchmarkGetDestinationURLStr_Optimized(b *testing.B) {
	r := &gateway.Route{
		URI: "https://backend.local/api",
	}
	u := &url.URL{
		Path:     "/users",
		RawQuery: "id=123&active=true",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.GetDestinationURL(u)
	}
}
