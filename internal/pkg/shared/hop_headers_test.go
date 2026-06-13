package shared_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

func TestRemoveHopByHopHeaders(t *testing.T) {
	tests := []struct {
		header   http.Header
		expected http.Header
		name     string
	}{
		{
			name: "well-known hop-by-hop headers are removed",
			header: http.Header{
				"Connection":          {"keep-alive"},
				"Proxy-Connection":    {"keep-alive"},
				"Keep-Alive":          {"timeout=5"},
				"Proxy-Authenticate":  {"Basic"},
				"Proxy-Authorization": {"Basic Zm9v"},
				"Te":                  {"trailers"},
				"Trailer":             {"X-Checksum"},
				"Transfer-Encoding":   {"chunked"},
				"Upgrade":             {"websocket"},
				"Content-Type":        {"application/json"},
			},
			expected: http.Header{
				"Content-Type": {"application/json"},
			},
		},
		{
			name: "headers nominated by the Connection header are removed",
			header: http.Header{
				"Connection":   {"close, X-Custom-Hop"},
				"X-Custom-Hop": {"value"},
				"X-Request-Id": {"abc123"},
			},
			expected: http.Header{
				"X-Request-Id": {"abc123"},
			},
		},
		{
			name: "end-to-end headers are kept",
			header: http.Header{
				"Authorization": {"Bearer token"},
				"Content-Type":  {"application/json"},
				"Accept":        {"*/*"},
			},
			expected: http.Header{
				"Authorization": {"Bearer token"},
				"Content-Type":  {"application/json"},
				"Accept":        {"*/*"},
			},
		},
		{
			name:     "empty header map is a no-op",
			header:   http.Header{},
			expected: http.Header{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shared.RemoveHopByHopHeaders(tt.header)

			if !reflect.DeepEqual(tt.expected, tt.header) {
				t.Errorf("expected header %v actual %v", tt.expected, tt.header)
			}
		})
	}
}
