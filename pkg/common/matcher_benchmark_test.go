package common_test

import (
	"regexp"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
)

func BenchmarkPathMatcher(b *testing.B) {
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
		b.Run(c.pattern+"_"+c.path, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				common.PathMatcher(c.pattern, c.path)
			}
		})
	}
}

func BenchmarkHostMatcher(b *testing.B) {
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
	}

	for _, tc := range cases {
		pattern := regexp.MustCompile(common.ConvertPatternToRegex(tc.pattern))
		b.Run(tc.pattern+"_"+tc.host, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				common.HostMatcher(pattern, tc.host)
			}
		})
	}
}
