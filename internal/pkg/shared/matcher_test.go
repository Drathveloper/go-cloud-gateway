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

func TestPathPattern_MatchEscaped(t *testing.T) {
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
			name:    "empty match should fail when path is not empty",
			pattern: "",
			path:    "/a%2Fb",
			match:   false,
		},
		{
			name:    "any match should succeed",
			pattern: "/**",
			path:    "/a%2Fb/c",
			match:   true,
		},
		{
			name:    "path without escapes should match as it does decoded",
			pattern: "/server/**/x",
			path:    "/server/one/two/x",
			match:   true,
		},
		{
			name:    "simple match should succeed when the encoded separator is inside the segment",
			pattern: "/static/*.css",
			path:    "/static/a%2Fb.css",
			match:   true,
		},
		{
			name:    "simple match should succeed when the encoded separator is in lower case",
			pattern: "/static/*.css",
			path:    "/static/a%2fb.css",
			match:   true,
		},
		{
			name:    "pattern with one more segment should fail when the separator is encoded",
			pattern: "/static/*/b.css",
			path:    "/static/a%2Fb.css",
			match:   false,
		},
		{
			name:    "literal pattern should fail when the separator is encoded",
			pattern: "/static/a/b.css",
			path:    "/static/a%2Fb.css",
			match:   false,
		},
		{
			name:    "single character match should succeed on an encoded separator",
			pattern: "/static/a?b.css",
			path:    "/static/a%2Fb.css",
			match:   true,
		},
		{
			name:    "single character match should take an escape as one character",
			pattern: "/files/my?file",
			path:    "/files/my%20file",
			match:   true,
		},
		{
			name:    "single character match should fail when the escape is taken as three characters",
			pattern: "/files/my???file",
			path:    "/files/my%20file",
			match:   false,
		},
		{
			name:    "literal pattern should match the character an escape stands for",
			pattern: "/files/my file",
			path:    "/files/my%20file",
			match:   true,
		},
		{
			name:    "literal pattern should match an escaped unreserved character",
			pattern: "/files/abc",
			path:    "/files/%61b%63",
			match:   true,
		},
		{
			name:    "literal pattern should not match the text of an escape",
			pattern: "/files/my%20file",
			path:    "/files/my%20file",
			match:   false,
		},
		{
			name:    "literal pattern should match an escaped percent sign",
			pattern: "/files/100%",
			path:    "/files/100%25",
			match:   true,
		},
		{
			name:    "escaped percent sign followed by 2F should not be an encoded separator",
			pattern: "/files/a%2Fb",
			path:    "/files/a%252Fb",
			match:   true,
		},
		{
			name:    "simple match should backtrack over escapes",
			pattern: "/files/*%20b",
			path:    "/files/a%2520%2520b%2520b",
			match:   true,
		},
		{
			name:    "simple match should fail when nothing follows the wildcard match",
			pattern: "/files/*b",
			path:    "/files/a%2Fb%2Fc",
			match:   false,
		},
		{
			name:    "recursive match should count an encoded separator as part of one segment",
			pattern: "/api/**/users/*",
			path:    "/api/v1/users/extra/users/7%2F8",
			match:   true,
		},
		{
			name:    "recursive match should fail when the last segment is behind an encoded separator",
			pattern: "/api/**/users",
			path:    "/api/v1%2Fusers",
			match:   false,
		},
		{
			name:    "leading encoded separator should not be trimmed",
			pattern: "/a",
			path:    "/%2Fa",
			match:   false,
		},
		{
			name:    "trailing encoded separator should not be trimmed",
			pattern: "/a",
			path:    "/a%2F",
			match:   false,
		},
		{
			name:    "trailing separator should not be significant",
			pattern: "/a%2Fb",
			path:    "/a%252Fb/",
			match:   true,
		},
		{
			name:    "encoded dot segment should stay one segment with the separator",
			pattern: "/public/*",
			path:    "/public/..%2Fadmin",
			match:   true,
		},
		{
			name:    "encoded dot segment should not be seen as the segments it decodes to",
			pattern: "/public/*/admin",
			path:    "/public/..%2Fadmin",
			match:   false,
		},
		{
			name:    "percent sign that does not start an escape should be taken as it is",
			pattern: "/a%zz/b%2",
			path:    "/a%zz/b%2",
			match:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := shared.CompilePathPattern(tt.pattern)

			actual := pattern.MatchEscaped(tt.path)
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

// A '*' of the pattern is a wildcard whatever the segment has at that place, a '*' of
// its own included: it used to be compared as a literal there, and the rest of the
// pattern was then held against a segment that the wildcard would have matched.
func TestPathPattern_Match_WildcardOverALiteralAsterisk(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		escaped string
		match   bool
	}{
		{
			name:    "wildcard should match a segment that starts with an asterisk",
			pattern: "/files/*.css",
			path:    "/files/*x.css",
			escaped: "/files/%2Ax.css",
			match:   true,
		},
		{
			name:    "wildcard should match an asterisk and what follows it",
			pattern: "/a*",
			path:    "/a*b",
			escaped: "/a%2Ab",
			match:   true,
		},
		{
			name:    "wildcard should match a lone asterisk",
			pattern: "/a/*",
			path:    "/a/*",
			escaped: "/a/%2a",
			match:   true,
		},
		{
			name:    "wildcard between literals should match a segment full of asterisks",
			pattern: "/a*b",
			path:    "/a***b",
			escaped: "/a%2A*%2Ab",
			match:   true,
		},
		{
			name:    "literal after the wildcard should still be required",
			pattern: "/files/*.css",
			path:    "/files/*x.js",
			escaped: "/files/%2Ax.js",
			match:   false,
		},
		{
			name:    "wildcard should not cross a separator after an asterisk",
			pattern: "/files/*.css",
			path:    "/files/*/x.css",
			escaped: "/files/%2A/x.css",
			match:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := shared.CompilePathPattern(tt.pattern)

			if actual := pattern.Match(tt.path); actual != tt.match {
				t.Errorf("path %q: expected %t actual %t", tt.path, tt.match, actual)
			}
			for _, escaped := range []string{tt.path, tt.escaped} {
				if actual := pattern.MatchEscaped(escaped); actual != tt.match {
					t.Errorf("escaped path %q: expected %t actual %t", escaped, tt.match, actual)
				}
			}
		})
	}
}
