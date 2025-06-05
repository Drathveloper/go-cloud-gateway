package gateway_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func BenchmarkNewGatewayRequest(b *testing.B) {
	body := []byte(`{"message":"hello world"}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := &http.Request{
			Method: "POST",
			URL:    &url.URL{Scheme: "http", Host: "localhost", Path: "/api"},
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   io.NopCloser(bytes.NewReader(body)),
		}
		r, err := gateway.NewGatewayRequest(req)
		if err != nil || r.Body == nil {
			b.Fatal("failed to create gateway request")
		}
	}
}

func BenchmarkNewGatewayRequest_LargeBody(b *testing.B) {
	largeBody := bytes.Repeat([]byte("x"), 10_000) // 10KB
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := &http.Request{
			Method: "POST",
			URL:    &url.URL{Scheme: "http", Host: "localhost", Path: "/api"},
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   io.NopCloser(bytes.NewReader(largeBody)),
		}
		r, err := gateway.NewGatewayRequest(req)
		if err != nil || r.Body == nil {
			b.Fatal("failed to create gateway request")
		}
	}
}

func BenchmarkNewGatewayResponse(b *testing.B) {
	body := []byte(`{"message":"hello world"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}
		r, err := gateway.NewGatewayResponse(res)
		if err != nil || r.Body == nil {
			b.Fatal("failed to create gateway response")
		}
	}
}

func BenchmarkNewGatewayResponse_LargeBody(b *testing.B) {
	const bodySize = 10 * 1024 // 10 KB
	body := bytes.Repeat([]byte("A"), bodySize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}
		r, err := gateway.NewGatewayResponse(res)
		if err != nil || len(r.Body) != bodySize {
			b.Fatal("failed to create gateway response with large body")
		}
	}
}
