package common_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
)

func BenchmarkReadBody(b *testing.B) {
	testCases := []struct {
		name  string
		input string
	}{
		{"Empty", ""},
		{"Small", "hello world"},
		{"Medium", strings.Repeat("a", 1024)},            // 1KB
		{"Large", strings.Repeat("b", 1024*1024)},        // 1MB
		{"VeryLarge", strings.Repeat("c", 10*1024*1024)}, // 10MB
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			var err error
			readCloser := io.NopCloser(bytes.NewReader([]byte(tc.input)))

			b.ResetTimer()
			for range b.N {
				if _, err = common.ReadBody(readCloser); err != nil {
					b.Fatalf("ReadBody failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkReadBody_Nil(b *testing.B) {
	for range b.N {
		_, err := common.ReadBody(nil)
		if err != nil {
			b.Fatalf("ReadBody failed: %v", err)
		}
	}
}

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
			common.WriteHeader(rec, simpleHeader)
		}
	})

	b.Run("MultiValueHeader", func(b *testing.B) {
		for range b.N {
			rec := httptest.NewRecorder()

			b.ResetTimer()
			common.WriteHeader(rec, multiValueHeader)
		}
	})
}
