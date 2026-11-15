package filter

import (
	"log/slog"
	"net/http"
	"reflect"
	"testing"
)

func TestRequestResponseLogger_RedactedQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "redacted query should return the same query when it is empty",
			query:    "",
			expected: "",
		},
		{
			name:     "redacted query should return the same query when no parameter is sensitive",
			query:    "page=1&size=10&sort=name,asc&flag&filter%5Bname%5D=a+b&Page=2",
			expected: "page=1&size=10&sort=name,asc&flag&filter%5Bname%5D=a+b&Page=2",
		},
		{
			name:     "redacted query should return the same query when the sensitive parameters have no value",
			query:    "token&secret=&page=1",
			expected: "token&secret=&page=1",
		},
		{
			name:     "redacted query should return the same query when it only holds separators",
			query:    "&;&&;",
			expected: "&;&&;",
		},
		{
			name:     "redacted query should replace the value when a parameter is sensitive",
			query:    "page=1&access_token=abc&size=10",
			expected: "page=1&access_token=[REDACTED]&size=10",
		},
		{
			name:     "redacted query should replace the value when the sensitive parameter is the first one",
			query:    "token=abc&page=1",
			expected: "token=[REDACTED]&page=1",
		},
		{
			name:     "redacted query should replace the value when the sensitive parameter is the last one",
			query:    "page=1&token=abc",
			expected: "page=1&token=[REDACTED]",
		},
		{
			name:     "redacted query should replace the value when it is followed by a separator",
			query:    "token=abc&",
			expected: "token=[REDACTED]&",
		},
		{
			name:     "redacted query should replace the value when the value is already the mark",
			query:    "token=[REDACTED]&page=1",
			expected: "token=[REDACTED]&page=1",
		},
		{
			name:     "redacted query should replace the value when the encoded name ends the query",
			query:    "page=1&%74oken=%",
			expected: "page=1&%74oken=[REDACTED]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewHeadersOnlyRequestResponseLoggerFilter(slog.LevelInfo)

			actual := f.redactedQuery(tt.query)

			if tt.expected != actual {
				t.Errorf("expected %s actual %s", tt.expected, actual)
			}
		})
	}
}

func TestRequestResponseLogger_RedactedQuery_DoesNotAllocateWhenNothingIsSensitive(t *testing.T) {
	queries := []string{
		"",
		"page=1&size=10&sort=name,asc&flag&Page=2",
		// Encoded and upper case names are compared without building the decoded name.
		"filter%5Bname%5D=a+b&ACCESS%5FTOKEN_HINT=x&a%20very%20long%20name%20that%20is%20not%20sensitive=1",
		"token&secret=&page=1;sig=abc",
	}
	f := NewHeadersOnlyRequestResponseLoggerFilter(slog.LevelInfo).WithRedactedQueryParams("session_id", "api key")
	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			allocs := testing.AllocsPerRun(100, func() {
				if actual := f.redactedQuery(query); actual != query {
					t.Errorf("expected %s actual %s", query, actual)
				}
			})

			if allocs != 0 {
				t.Errorf("expected %v actual %v", 0, allocs)
			}
		})
	}
}

func TestRequestResponseLogger_RedactedCopy(t *testing.T) {
	tests := []struct {
		headers    http.Header
		expected   http.Header
		name       string
		expectCopy bool
	}{
		{
			name:       "redacted copy should return the same map when there are no headers",
			headers:    nil,
			expected:   nil,
			expectCopy: false,
		},
		{
			name:       "redacted copy should return the same map when no header is sensitive",
			headers:    http.Header{"Accept": {"text/plain"}, "X-Request-Id": {"1"}},
			expected:   http.Header{"Accept": {"text/plain"}, "X-Request-Id": {"1"}},
			expectCopy: false,
		},
		{
			name:       "redacted copy should return another map when a header is sensitive",
			headers:    http.Header{"Accept": {"text/plain"}, "Authorization": {"Bearer super-secret"}},
			expected:   http.Header{"Accept": {"text/plain"}, "Authorization": {redactedValue}},
			expectCopy: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.headers.Clone()
			f := NewHeadersOnlyRequestResponseLoggerFilter(slog.LevelInfo)

			actual := f.redactedCopy(tt.headers)

			if !reflect.DeepEqual(tt.expected, actual) {
				t.Errorf("expected %v actual %v", tt.expected, actual)
			}
			isCopy := reflect.ValueOf(actual).Pointer() != reflect.ValueOf(tt.headers).Pointer()
			if tt.expectCopy != isCopy {
				t.Errorf("expected %t actual %t", tt.expectCopy, isCopy)
			}
			if !reflect.DeepEqual(original, tt.headers) {
				t.Errorf("expected %v actual %v", original, tt.headers)
			}
		})
	}
}
