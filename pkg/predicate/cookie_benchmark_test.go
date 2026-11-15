package predicate_test

import (
	"net/http"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func BenchmarkCookiePredicate_Test(b *testing.B) {
	benchmarks := []struct {
		header   http.Header
		name     string
		expected bool
	}{
		{
			name: "Match",
			header: http.Header{
				"Accept": {"application/json"},
				"Cookie": {"theme=dark; lang=en; consent=yes; tracking=4f9d1c; SessionId=8842"},
			},
			expected: true,
		},
		{
			name: "NoMatch",
			header: http.Header{
				"Accept": {"application/json"},
				"Cookie": {"theme=dark; lang=en; consent=yes; tracking=4f9d1c; visitor=8842"},
			},
			expected: false,
		},
		{
			name: "NoCookieHeader",
			header: http.Header{
				"Accept": {"application/json"},
			},
			expected: false,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			p, err := predicate.NewCookiePredicate("SessionId", "^[0-9]+$")
			if err != nil {
				b.Fatalf("failed to create predicate: %v", err)
			}
			req := &http.Request{Header: bm.header}

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
