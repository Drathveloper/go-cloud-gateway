package predicate_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func BenchmarkPathPredicate_Test(b *testing.B) {
	benchmarks := []struct {
		name     string
		target   string
		expected bool
	}{
		{
			// the common request: nothing encoded, so the url has no raw path
			name:     "Plain",
			target:   "/api/v1/tenants/42/users/7",
			expected: true,
		},
		{
			name:     "PlainNoMatch",
			target:   "/api/v1/tenants/42/orders/7",
			expected: false,
		},
		{
			// encoded the way net/url would do it: the url has no raw path either
			name:     "EncodedWithoutRawPath",
			target:   "/api/v1/tenants/42/users/my%20user",
			expected: true,
		},
		{
			// an encoding net/url would not produce sets the raw path, but there is no encoded separator
			name:     "RawPathWithoutEncodedSeparator",
			target:   "/api/v1/tenants/42/users/%6Dy%20user",
			expected: true,
		},
		{
			name:     "EncodedSeparator",
			target:   "/api/v1/tenants/42/users/my%2Fuser",
			expected: true,
		},
		{
			name:     "EncodedSeparatorNoMatch",
			target:   "/api/v1/tenants/42/orders/my%2Forder",
			expected: false,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			p := predicate.NewPathPredicate("/api/*/orders", "/api/**/users/*")
			// parsed the way the server parses the request target
			requestURL, err := url.ParseRequestURI(bm.target)
			if err != nil {
				b.Fatalf("failed to parse request target: %v", err)
			}
			req := &http.Request{URL: requestURL}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if p.Test(req) != bm.expected {
					b.Fatalf("expected %t actual %t", bm.expected, !bm.expected)
				}
			}
		})
	}
}
