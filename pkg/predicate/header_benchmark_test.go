package predicate_test

import (
	"net/http"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func BenchmarkHeaderPredicate_Test(b *testing.B) {
	benchmarks := []struct {
		header   http.Header
		name     string
		expected bool
	}{
		{
			name: "Match",
			header: http.Header{
				"Accept":          {"application/json"},
				"Accept-Encoding": {"gzip, deflate, br"},
				"User-Agent":      {"benchmark/1.0"},
				"X-Request-Id":    {"7f3c2a1e-0b7d-4c55-9a51-2f6f1f0f2f11"},
				"X-Tenant-Id":     {"tenant-42"},
			},
			expected: true,
		},
		{
			name: "NoMatch",
			header: http.Header{
				"Accept":          {"application/json"},
				"Accept-Encoding": {"gzip, deflate, br"},
				"User-Agent":      {"benchmark/1.0"},
				"X-Request-Id":    {"7f3c2a1e-0b7d-4c55-9a51-2f6f1f0f2f11"},
				"X-Tenant-Id":     {"other"},
			},
			expected: false,
		},
		{
			name: "NoHeader",
			header: http.Header{
				"Accept":          {"application/json"},
				"Accept-Encoding": {"gzip, deflate, br"},
				"User-Agent":      {"benchmark/1.0"},
				"X-Request-Id":    {"7f3c2a1e-0b7d-4c55-9a51-2f6f1f0f2f11"},
			},
			expected: false,
		},
		{
			// the value does not match as a whole: the element of the list does
			name: "ListMatch",
			header: http.Header{
				"Accept":          {"application/json"},
				"Accept-Encoding": {"gzip, deflate, br"},
				"User-Agent":      {"benchmark/1.0"},
				"X-Request-Id":    {"7f3c2a1e-0b7d-4c55-9a51-2f6f1f0f2f11"},
				"X-Tenant-Id":     {"other, \"quoted, other\", tenant-42"},
			},
			expected: true,
		},
		{
			// the worst case: the whole value and every element of the list are tested
			name: "ListNoMatch",
			header: http.Header{
				"Accept":          {"application/json"},
				"Accept-Encoding": {"gzip, deflate, br"},
				"User-Agent":      {"benchmark/1.0"},
				"X-Request-Id":    {"7f3c2a1e-0b7d-4c55-9a51-2f6f1f0f2f11"},
				"X-Tenant-Id":     {"other, \"quoted, other\", another"},
			},
			expected: false,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// the name is configured in lower case on purpose: configuration files rarely use the canonical form
			p, err := predicate.NewHeaderPredicate("x-tenant-id", "^tenant-[0-9]+$")
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

func BenchmarkHeaderPredicate_Test_Host(b *testing.B) {
	benchmarks := []struct {
		name     string
		host     string
		expected bool
	}{
		{
			name:     "Match",
			host:     "tenant-42.example.org:8443",
			expected: true,
		},
		{
			name:     "NoMatch",
			host:     "www.example.org:8443",
			expected: false,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			p, err := predicate.NewHeaderPredicate("host", `^tenant-[0-9]+\.example\.org(:[0-9]+)?$`)
			if err != nil {
				b.Fatalf("failed to create predicate: %v", err)
			}
			// the server leaves the host here and not in the headers
			req := &http.Request{Host: bm.host, Header: http.Header{"Accept": {"application/json"}}}

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
