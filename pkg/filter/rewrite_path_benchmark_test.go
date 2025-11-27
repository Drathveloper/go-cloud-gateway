package filter_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func BenchmarkRewritePath_PreProcess(b *testing.B) {
	benchmarks := []struct {
		name        string
		regexp      string
		replacement string
		originalURL string
	}{
		{
			name:        "SimpleReplace",
			regexp:      "^/foo",
			replacement: "/bar",
			originalURL: "http://localhost/foo/test",
		},
		{
			name:        "NoMatch",
			regexp:      "^/baz",
			replacement: "/qux",
			originalURL: "http://localhost/foo/test",
		},
		{
			name:        "WithGroups",
			regexp:      "^/user/(\\d+)",
			replacement: "/profile/$1",
			originalURL: "http://localhost/user/123",
		},
		{
			name:        "DeepPath",
			regexp:      "^/api/v1/(.*)",
			replacement: "/v2/$1",
			originalURL: "http://localhost/api/v1/resource/item/1",
		},
		{
			name:        "LongPath",
			regexp:      "/segment",
			replacement: "/s",
			originalURL: "http://localhost/" + generateLongPath("/segment", 50),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			rewrite, err := filter.NewRewritePathFilter(bm.regexp, bm.replacement)
			if err != nil {
				b.Fatalf("Failed to create filter: %v", err)
			}

			u, err := url.Parse(bm.originalURL)
			if err != nil {
				b.Fatalf("Invalid URL: %v", err)
			}

			ctx := &gateway.Context{
				Request: &gateway.Request{
					URL: u,
				},
				Attributes: make(map[string]any),
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				_ = rewrite.PreProcess(ctx)
			}
		})
	}
}

func generateLongPath(segment string, repeat int) string {
	strBuilder := strings.Builder{}
	for range repeat {
		_, _ = strBuilder.WriteString(segment)
	}
	return strBuilder.String()
}
