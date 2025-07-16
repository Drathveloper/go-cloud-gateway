package gatewayhandler_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func BenchmarkServeHTTP_HappyPath(b *testing.B) {
	uri, _ := url.Parse("http://localhost")
	pred := predicate.NewPathPredicate("/test")
	route := gateway.Route{
		ID:  "test",
		URI: uri,
		Predicates: []gateway.Predicate{
			pred,
		},
	}
	body := []byte(`{"status":"ok"}`)
	handler := gatewayhandler.NewGatewayHandler(
		&mockGateway{
			doFunc: func(ctx *gateway.Context) error {
				ctx.Response = &gateway.Response{
					Status:     http.StatusOK,
					Headers:    http.Header{"Content-Type": {"application/json"}},
					BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer(body)), int64(len(body))),
				}
				return nil
			},
		},
		gateway.Routes{route},
		&mockErrorHandler{},
	)

	reqBody := []byte(`GET /test HTTP/1.1`)
	for range b.N {
		req := httptest.NewRequest(http.MethodGet, "/test?x=1", bytes.NewReader(reqBody))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		res := rec.Result()
		if res.StatusCode != http.StatusOK {
			b.Fatalf("unexpected status: %d", res.StatusCode)
		}
	}
}

func BenchmarkServeHTTP_RouteNotFound(b *testing.B) {
	uri, _ := url.Parse("http://localhost")
	pred := predicate.NewPathPredicate("/test")
	route := gateway.Route{
		ID:  "test",
		URI: uri,
		Predicates: []gateway.Predicate{
			pred,
		},
	}
	gwHandler := gatewayhandler.NewGatewayHandler(
		&mockGateway{},
		gateway.Routes{route},
		&mockErrorHandler{handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {}},
	)

	req := httptest.NewRequest(http.MethodGet, "/not-found", nil)

	b.ResetTimer()
	for range b.N {
		w := httptest.NewRecorder()
		gwHandler.ServeHTTP(w, req.Clone(req.Context()))
	}
}

func BenchmarkServeHTTP_BackendError(b *testing.B) {
	uri, _ := url.Parse("http://localhost")
	pred := predicate.NewPathPredicate("/test")
	route := gateway.Route{
		ID:  "test",
		URI: uri,
		Predicates: []gateway.Predicate{
			pred,
		},
	}

	gwHandler := gatewayhandler.NewGatewayHandler(
		&mockGateway{
			doFunc: func(_ *gateway.Context) error {
				return errors.New("backend error")
			},
		},
		gateway.Routes{route},
		&mockErrorHandler{handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {}},
	)

	req := httptest.NewRequest(http.MethodGet, "/fail", nil)

	b.ResetTimer()
	for range b.N {
		w := httptest.NewRecorder()
		gwHandler.ServeHTTP(w, req.Clone(req.Context()))
	}
}

func BenchmarkServeHTTP_LargeBody(b *testing.B) {
	uri, _ := url.Parse("http://localhost:8080")
	route := gateway.Route{
		ID:  "route-large",
		URI: uri,
	}
	largeBody := strings.Repeat("a", 1024*1024) // 1MB
	body := []byte(`{"status":"ok"}`)
	gwHandler := gatewayhandler.NewGatewayHandler(
		&mockGateway{
			doFunc: func(ctx *gateway.Context) error {
				ctx.Response = &gateway.Response{
					Status:     http.StatusOK,
					Headers:    http.Header{"Content-Type": {"application/json"}},
					BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer(body)), int64(len(body))),
				}
				return nil
			},
		},
		gateway.Routes{route},
		&mockErrorHandler{handleFunc: func(_ *gateway.Context, _ error, _ http.ResponseWriter) {}},
	)

	b.ResetTimer()
	for range b.N {
		req := httptest.NewRequest(http.MethodPost, "/big", io.NopCloser(strings.NewReader(largeBody)))
		w := httptest.NewRecorder()
		gwHandler.ServeHTTP(w, req.Clone(req.Context()))
	}
}
