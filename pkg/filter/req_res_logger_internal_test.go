package filter

import (
	"log/slog"
	"net/http"
	"reflect"
	"testing"
)

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
			expected:   http.Header{"Accept": {"text/plain"}, "Authorization": {redactedHeaderValue}},
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
