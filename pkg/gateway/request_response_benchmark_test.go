package gateway_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func BenchmarkNewGatewayRequest(b *testing.B) {
	body := []byte(`{"message":"hello world"}`)
	b.ResetTimer()
	for range b.N {
		req := &http.Request{
			Method: http.MethodPost,
			URL:    &url.URL{Scheme: "http", Host: "localhost", Path: "/api"},
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   io.NopCloser(bytes.NewReader(body)),
		}
		r, err := gateway.NewGatewayRequest(req)
		if err != nil || r.BodyReader == nil {
			b.Fatal("failed to create gateway request")
		}
	}
}

func BenchmarkNewGatewayRequest_LargeBody(b *testing.B) {
	largeBody := bytes.Repeat([]byte("x"), 10_000) // 10KB
	b.ResetTimer()
	for range b.N {
		req := &http.Request{
			Method: http.MethodPost,
			URL:    &url.URL{Scheme: "http", Host: "localhost", Path: "/api"},
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   io.NopCloser(bytes.NewReader(largeBody)),
		}
		r, err := gateway.NewGatewayRequest(req)
		if err != nil || r.BodyReader == nil {
			b.Fatal("failed to create gateway request")
		}
	}
}

func BenchmarkNewGatewayResponse(b *testing.B) {
	body := []byte(`{"message":"hello world"}`)

	b.ResetTimer()
	for range b.N {
		res := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}
		r := gateway.NewGatewayResponse(res)
		if r.BodyReader == nil {
			b.Fatal("failed to create gateway response")
		}
	}
}

func BenchmarkNewGatewayResponse_LargeBody(b *testing.B) {
	const bodySize = 10 * 1024 // 10 KB
	body := bytes.Repeat([]byte("A"), bodySize)

	b.ResetTimer()
	for range b.N {
		res := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}
		_ = gateway.NewGatewayResponse(res)
	}
}

func BenchmarkReplayableBody_Capture(b *testing.B) {
	sizes := []int{
		0,                // empty body
		128,              // 128B
		1024,             // 1KB
		10 * 1024,        // 10KB
		100 * 1024,       // 100KB
		1024 * 1024,      // 1MB
		10 * 1024 * 1024, // 10MB
	}

	for _, size := range sizes {
		b.Run(strconv.Itoa(size)+"_bytes", func(b *testing.B) {
			bodyData := bytes.Repeat([]byte("a"), size)
			for range b.N {
				r := io.NopCloser(bytes.NewReader(bodyData))
				rb := gateway.NewReplayableBody(r, int64(len(bodyData)))
				_ = rb.Capture()
			}
		})
	}
}
