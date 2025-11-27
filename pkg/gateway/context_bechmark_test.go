package gateway_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func BenchmarkNewGatewayContext(b *testing.B) {
	route := &gateway.Route{
		Timeout: 5 * time.Second,
		Logger:  slog.Default(),
	}

	req := &gateway.Request{}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		ctx, cancel := gateway.NewGatewayContext(route, req)
		cancel()
		gateway.ReleaseGatewayContext(ctx)
	}
}
