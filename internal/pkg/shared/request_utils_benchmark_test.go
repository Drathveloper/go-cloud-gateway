package shared_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

func BenchmarkWriteHeader(b *testing.B) {
	simpleHeader := http.Header{
		"X-Test":        {"value1"},
		"X-Request-ID":  {"abc123"},
		"X-Trace-ID":    {"xyz789"},
		"Content-Type":  {"application/json"},
		"Cache-Control": {"no-cache"},
	}

	multiValueHeader := http.Header{
		"Set-Cookie": {"a=1; Path=/", "b=2; Path=/"},
		"X-Test":     {"value1", "value2", "value3"},
	}

	b.Run("SimpleHeader", func(b *testing.B) {
		for range b.N {
			rec := httptest.NewRecorder()

			b.ResetTimer()
			shared.WriteHeader(rec, simpleHeader)
		}
	})

	b.Run("MultiValueHeader", func(b *testing.B) {
		for range b.N {
			rec := httptest.NewRecorder()

			b.ResetTimer()
			shared.WriteHeader(rec, multiValueHeader)
		}
	})
}
