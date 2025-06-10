package common_test

import (
	"regexp"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
)

func TestPathMatcher(t *testing.T) {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := common.PathMatcher(tt.pattern, tt.path)
			if tt.match != actual {
				t.Errorf("expected %t actual %t", tt.match, actual)
			}
		})
	}
}

func TestHostMatcher(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		host     string
		expected bool
	}{
		{
			name:     "host matcher should return true when pattern is not regex and matches",
			pattern:  "exact.host.com",
			host:     "exact.host.com",
			expected: true,
		},
		{
			name:     "host matcher should return false when pattern is not regex and doesn't match",
			pattern:  "exact.host.com",
			host:     "other.host.com",
			expected: false,
		},
		{
			name:     "host matcher should return true when pattern is single * regex and match",
			pattern:  "api.*.com",
			host:     "api.pokemon.com",
			expected: true,
		},
		{
			name:     "host matcher should return false when pattern is single * regex and doesn't match",
			pattern:  "api.*.com",
			host:     "api.pokemon.server.com",
			expected: false,
		},
		{
			name:     "host matcher should return true when pattern is ** regex and match",
			pattern:  "api.**.com",
			host:     "api.pokemon.server.com",
			expected: true,
		},
		{
			name:     "host matcher should return true when pattern is ** regex and doesn't match",
			pattern:  "api.**.com",
			host:     "pokemon.api.server.com",
			expected: false,
		},
		{
			name:     "host matcher should return true when pattern is full **",
			pattern:  "**",
			host:     "any.com",
			expected: true,
		},
		{
			name:     "host matcher should return true when pattern is nil",
			pattern:  "**",
			host:     "any.com",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pattern *regexp.Regexp
			if tt.pattern == "**" {
				pattern = nil
			} else {
				pattern = regexp.MustCompile(common.ConvertPatternToRegex(tt.pattern))
			}
			result := common.HostMatcher(pattern, tt.host)
			if tt.expected != result {
				t.Errorf("expected %t actual %t", tt.expected, result)
			}
		})
	}
}
