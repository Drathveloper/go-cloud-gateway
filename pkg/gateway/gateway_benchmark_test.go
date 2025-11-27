package gateway_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

type dummyHTTPClient struct {
	fail bool
}

func (c *dummyHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	if c.fail {
		return nil, errors.New("http client error")
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       http.NoBody,
	}, nil
}

func newDummyContext() *gateway.Context {
	reqURL, _ := url.Parse("https://example.com")
	req := &gateway.Request{
		Method:     http.MethodGet,
		URL:        reqURL,
		Headers:    make(http.Header),
		BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer([]byte("payload"))), int64(len("payload"))),
	}
	routeURL, _ := url.Parse("https://example.com/test")
	route := &gateway.Route{
		ID:      "route-1",
		Timeout: 1000000000, // 1s
		URI:     routeURL,
	}
	ctx, _ := gateway.NewGatewayContext(route, req)
	return ctx
}

func BenchmarkGatewayDo_Success(b *testing.B) {
	g := gateway.NewGateway(&dummyHTTPClient{})
	b.ResetTimer()
	for b.Loop() {
		ctx := newDummyContext()
		_ = g.Do(ctx)
		gateway.ReleaseGatewayContext(ctx)
	}
}

func BenchmarkGatewayDo_HTTPError(b *testing.B) {
	g := gateway.NewGateway(&dummyHTTPClient{fail: true})
	b.ResetTimer()
	for b.Loop() {
		ctx := newDummyContext()
		_ = g.Do(ctx)
		gateway.ReleaseGatewayContext(ctx)
	}
}

func BenchmarkGatewayDo_PreFilterError(b *testing.B) {
	g := gateway.NewGateway(&dummyHTTPClient{})
	b.ResetTimer()
	for b.Loop() {
		ctx := newDummyContext()
		_ = g.Do(ctx)
		gateway.ReleaseGatewayContext(ctx)
	}
}

func BenchmarkGatewayDo_PostFilterError(b *testing.B) {
	g := gateway.NewGateway(&dummyHTTPClient{})
	b.ResetTimer()
	for b.Loop() {
		ctx := newDummyContext()
		_ = g.Do(ctx)
		gateway.ReleaseGatewayContext(ctx)
	}
}
