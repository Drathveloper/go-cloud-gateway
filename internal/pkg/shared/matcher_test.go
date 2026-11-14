package shared_test

import (
	"testing"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

func TestPathPattern_Match(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		match   bool
	}{
		{
			name:    "empty match should succeed when path is empty",
			pattern: "",
			path:    "",
			match:   true,
		},
		{
			name:    "recursive match should succeed with one nesting level",
			pattern: "/server/one/**",
			path:    "/server/one/x",
			match:   true,
		},
		{
			name:    "recursive match should succeed with multiple nesting level",
			pattern: "/server/one/**",
			path:    "/server/one/x/t",
			match:   true,
		},
		{
			name:    "simple match should succeed with single nesting level",
			pattern: "/server/one/*",
			path:    "/server/one/x",
			match:   true,
		},
		{
			name:    "simple match should fail with multiple nesting level",
			pattern: "/server/one/*",
			path:    "/server/one/x/t",
			match:   false,
		},
		{
			name:    "single character match should succeed",
			pattern: "/server/?ne/**",
			path:    "/server/one/x",
			match:   true,
		},
		{
			name:    "recursive match should succeed when ends with other path",
			pattern: "/server/**/x",
			path:    "/server/one/two/x",
			match:   true,
		},
		{
			name:    "any match should succeed",
			pattern: "/**",
			path:    "/any/route",
			match:   true,
		},
		{
			name:    "segment doesn't match",
			pattern: "/user/profile",
			path:    "/user/settings",
			match:   false,
		},
		{
			name:    "pattern longest than path",
			pattern: "/user/profile/details",
			path:    "/user/profile",
			match:   false,
		},
		{
			name:    "pattern with double asterisk but next doesn't match",
			pattern: "/user/**/details",
			path:    "/user/profile",
			match:   false,
		},
		{
			name:    "longest pattern than path after double asterisk",
			pattern: "/user/**/details/extra",
			path:    "/user/profile/details",
			match:   false,
		},
		{
			name:    "? matcher should match",
			pattern: "/user/?/extra",
			path:    "/user//extra",
			match:   false,
		},
		{
			name:    "empty pattern should not match a non-empty path",
			pattern: "",
			path:    "/",
			match:   false,
		},
		{
			name:    "recursive match should succeed with no nesting level",
			pattern: "/server/one/**",
			path:    "/server/one",
			match:   true,
		},
		{
			name:    "recursive match in the middle should succeed with no nesting level",
			pattern: "/server/**/x",
			path:    "/server/x",
			match:   true,
		},
		{
			name:    "recursive match should skip an earlier occurrence of the next segment",
			pattern: "/a/**/c/d",
			path:    "/a/c/x/c/d",
			match:   true,
		},
		{
			name:    "recursive match should skip an earlier occurrence followed by a wildcard",
			pattern: "/api/**/users/*",
			path:    "/api/v1/users/extra/users/7",
			match:   true,
		},
		{
			name:    "recursive match should be followed by a wildcard segment",
			pattern: "/a/**/b*",
			path:    "/a/x/bcd",
			match:   true,
		},
		{
			name:    "recursive match should fail when the segment after it never matches",
			pattern: "/a/**/b*",
			path:    "/a/x/cd",
			match:   false,
		},
		{
			name:    "several recursive matches should succeed",
			pattern: "/a/**/b/**/c",
			path:    "/a/x/b/y/b/z/c",
			match:   true,
		},
		{
			name:    "several recursive matches should fail when the order is wrong",
			pattern: "/a/**/b/**/c",
			path:    "/a/c/x/b",
			match:   false,
		},
		{
			name:    "recursive match without leading separator should match any path",
			pattern: "**",
			path:    "/any/route",
			match:   true,
		},
		{
			name:    "trailing separators should not be significant",
			pattern: "/server/one/",
			path:    "/server/one",
			match:   true,
		},
		{
			name:    "root pattern should match the root path",
			pattern: "/",
			path:    "/",
			match:   true,
		},
		{
			name:    "root pattern should not match another path",
			pattern: "/",
			path:    "/server",
			match:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := shared.CompilePathPattern(tt.pattern)

			actual := pattern.Match(tt.path)
			if tt.match != actual {
				t.Errorf("expected %t actual %t", tt.match, actual)
			}
		})
	}
}

func TestHostPattern_Match(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		host     string
		expected bool
	}{
		{
			name:     "literal pattern should match the same host",
			pattern:  "exact.host.com",
			host:     "exact.host.com",
			expected: true,
		},
		{
			name:     "literal pattern should not match another host",
			pattern:  "exact.host.com",
			host:     "other.host.com",
			expected: false,
		},
		{
			name:     "single * should match one label",
			pattern:  "api.*.com",
			host:     "api.pokemon.com",
			expected: true,
		},
		{
			name:     "single * should not match several labels",
			pattern:  "api.*.com",
			host:     "api.pokemon.server.com",
			expected: false,
		},
		{
			name:     "** should match several labels",
			pattern:  "api.**.com",
			host:     "api.pokemon.server.com",
			expected: true,
		},
		{
			name:     "** should not match when the rest of the pattern does not",
			pattern:  "api.**.com",
			host:     "pokemon.api.server.com",
			expected: false,
		},
		{
			name:     "full ** should match any host",
			pattern:  "**",
			host:     "any.com",
			expected: true,
		},
		{
			name:     "full ** should match an empty host",
			pattern:  "**",
			host:     "",
			expected: true,
		},
		{
			name:     "? should match exactly one character",
			pattern:  "api?.host.com",
			host:     "api1.host.com",
			expected: true,
		},
		{
			name:     "? should not match no character",
			pattern:  "api?.host.com",
			host:     "api.host.com",
			expected: false,
		},
		{
			name:     "? should not match a label separator",
			pattern:  "api?host.com",
			host:     "api.host.com",
			expected: false,
		},
		{
			name:     "literal pattern should ignore the port of the host",
			pattern:  "exact.host.com",
			host:     "exact.host.com:8080",
			expected: true,
		},
		{
			name:     "wildcard pattern should ignore the port of the host",
			pattern:  "**.host.com",
			host:     "api.host.com:8080",
			expected: true,
		},
		{
			name:     "pattern with port should match the same host and port",
			pattern:  "exact.host.com:8080",
			host:     "exact.host.com:8080",
			expected: true,
		},
		{
			name:     "pattern with port should not match another port",
			pattern:  "exact.host.com:8080",
			host:     "exact.host.com:9090",
			expected: false,
		},
		{
			name:     "pattern with port should not match a host without port",
			pattern:  "exact.host.com:8080",
			host:     "exact.host.com",
			expected: false,
		},
		{
			name:     "wildcard pattern with port should match the same port",
			pattern:  "*.host.com:8080",
			host:     "api.host.com:8080",
			expected: true,
		},
		{
			name:     "literal pattern should match regardless of the host case",
			pattern:  "exact.host.com",
			host:     "Exact.HOST.com",
			expected: true,
		},
		{
			name:     "wildcard pattern should match regardless of the pattern case",
			pattern:  "*.Host.COM",
			host:     "api.host.com",
			expected: true,
		},
		{
			name:     "dot should not match any character",
			pattern:  "*.host.com",
			host:     "api.hostxcom",
			expected: false,
		},
		{
			name:     "regular expression syntax should be literal",
			pattern:  "(api|web).*.com",
			host:     "api.host.com",
			expected: false,
		},
		{
			name:     "bracketed IPv6 pattern should ignore the port of the host",
			pattern:  "[::1]",
			host:     "[::1]:8080",
			expected: true,
		},
		{
			name:     "bracketed IPv6 pattern should match a host without port",
			pattern:  "[::1]",
			host:     "[::1]",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := shared.CompileHostPattern(tt.pattern)
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}

			result := pattern.Match(tt.host)
			if tt.expected != result {
				t.Errorf("expected %t actual %t", tt.expected, result)
			}
		})
	}
}
