package shared_test

import (
	"testing"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

func BenchmarkPathPattern_Match(b *testing.B) {
	cases := []struct {
		pattern string
		path    string
	}{
		{"/**", "/a/b/c"},
		{"/a/*", "/a/b"},
		{"/a/?", "/a/b"},
		{"/a/**/c", "/a/b/d/c"},
		{"/a/b/c", "/a/b/c"},
		{"/a/*/c", "/a/b/c"},
		{"/a/**", "/a/b/c/d/e"},
		{"/a/b", "/a/b/c"},
		{"", ""},
		{"/a/b/c", "/x/y/z"},
		{"/server/one/**", "/server/one/x/t"},
		{"/server/one/*", "/server/one/x/t"},
		{"/server/?ne/**", "/server/one/x"},
		{"/server/**/x", "/server/one/two/x"},
		{"/user/profile", "/user/settings"},
		{"/user/profile/details", "/user/profile"},
		{"/user/**/details", "/user/profile"},
		{"/user/**/details/extra", "/user/profile/details"},
		{"/user/?/extra", "/user//extra"},
	}

	for _, c := range cases {
		pattern := shared.CompilePathPattern(c.pattern)
		b.Run(c.pattern+"_"+c.path, func(b *testing.B) {
			for b.Loop() {
				pattern.Match(c.path)
			}
		})
	}
}

func BenchmarkHostPattern_Match(b *testing.B) {
	cases := []struct {
		pattern string
		host    string
	}{
		{"*", "example.com"},
		{"*.example.com", "sub.example.com"},
		{"**", "anything.goes.here"},
		{"api.*.example.com", "api.v1.example.com"},
		{"*.example.*", "sub.example.org"},
		{"a.*.b.*.c", "a.foo.b.bar.c"},
		{"api.example.com", "api.example.com"},
		{"api.example.com", "api.example.com:8080"},
		{"*.example.com", "sub.example.com:8080"},
	}

	for _, tc := range cases {
		pattern, err := shared.CompileHostPattern(tc.pattern)
		if err != nil {
			b.Fatalf("failed to compile host pattern: %v", err)
		}
		b.Run(tc.pattern+"_"+tc.host, func(b *testing.B) {
			for b.Loop() {
				pattern.Match(tc.host)
			}
		})
	}
}
