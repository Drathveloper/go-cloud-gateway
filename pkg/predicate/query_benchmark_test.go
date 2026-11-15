package predicate_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func BenchmarkQueryPredicate_Test(b *testing.B) {
	benchmarks := []struct {
		name     string
		rawQuery string
		expected bool
	}{
		{
			name:     "Match",
			rawQuery: "page=3&size=50&sort=created%2Cdesc&q=gateway+routes&version=v2",
			expected: true,
		},
		{
			name:     "NoMatch",
			rawQuery: "page=3&size=50&sort=created%2Cdesc&q=gateway+routes&lang=en",
			expected: false,
		},
		{
			name:     "NoQueryString",
			rawQuery: "",
			expected: false,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			p, err := predicate.NewQueryPredicate("version", "^v[0-9]+$")
			if err != nil {
				b.Fatalf("failed to create predicate: %v", err)
			}
			req := &http.Request{URL: &url.URL{Path: "/server/test", RawQuery: bm.rawQuery}}

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
