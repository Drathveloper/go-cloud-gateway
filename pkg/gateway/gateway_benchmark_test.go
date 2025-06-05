package gateway_test

import (
	"errors"
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
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       http.NoBody,
	}, nil
}

func newDummyContext() *gateway.Context {
	reqURL, _ := url.Parse("https://example.com")
	req := &gateway.Request{
		Method:  http.MethodGet,
		URL:     reqURL,
		Headers: make(http.Header),
		Body:    []byte("payload"),
	}
	route := &gateway.Route{
		ID:      "route-1",
		Timeout: 1000000000, // 1s
		URI:     "https://example.com/test",
	}
	ctx, _ := gateway.NewGatewayContext(route, req)
	return ctx
}

func BenchmarkGatewayDo_Success(b *testing.B) {
	g := gateway.NewGateway([]gateway.Filter{&DummyFilter{}}, &dummyHTTPClient{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := newDummyContext()
		_ = g.Do(ctx)
		gateway.ReleaseGatewayContext(ctx)
	}
}

func BenchmarkGatewayDo_HTTPError(b *testing.B) {
	g := gateway.NewGateway(nil, &dummyHTTPClient{fail: true})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := newDummyContext()
		_ = g.Do(ctx)
		gateway.ReleaseGatewayContext(ctx)
	}
}

func BenchmarkGatewayDo_PreFilterError(b *testing.B) {
	g := gateway.NewGateway([]gateway.Filter{&DummyFilter{PreProcessErr: errors.New("someErr")}}, &dummyHTTPClient{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := newDummyContext()
		_ = g.Do(ctx)
		gateway.ReleaseGatewayContext(ctx)
	}
}

func BenchmarkGatewayDo_PostFilterError(b *testing.B) {
	g := gateway.NewGateway([]gateway.Filter{&DummyFilter{PostProcessErr: errors.New("someErr")}}, &dummyHTTPClient{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := newDummyContext()
		_ = g.Do(ctx)
		gateway.ReleaseGatewayContext(ctx)
	}
}
