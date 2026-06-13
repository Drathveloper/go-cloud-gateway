package gateway_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func TestNewGatewayContext_ParentCancellationPropagates(t *testing.T) {
	parent, cancelParent := context.WithCancel(t.Context())
	route := &gateway.Route{ID: "r1", Timeout: time.Minute}

	ctx, cancel := gateway.NewGatewayContext(parent, route, &gateway.Request{})
	defer cancel()

	cancelParent()

	select {
	case <-ctx.Done():
	default:
		t.Fatal("expected gateway context to be done after parent cancellation")
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Errorf("expected context.Canceled, actual %v", ctx.Err())
	}
}

type wrapKey struct{}

func TestRouteFromContext(t *testing.T) {
	route := &gateway.Route{ID: "r1", Timeout: time.Minute}
	ctx, cancel := gateway.NewGatewayContext(t.Context(), route, &gateway.Request{})
	defer cancel()

	if got := gateway.RouteFromContext(ctx); got != route {
		t.Errorf("expected route from gateway context, actual %v", got)
	}
	wrapped := context.WithValue(ctx, wrapKey{}, "value")
	if got := gateway.RouteFromContext(wrapped); got != route {
		t.Errorf("expected route through a wrapped context, actual %v", got)
	}
	if got := gateway.RouteFromContext(t.Context()); got != nil {
		t.Errorf("expected nil route for a plain context, actual %v", got)
	}
}

func TestNewGatewayContext_ReusedContextHasEmptyAttributes(t *testing.T) {
	route := &gateway.Route{ID: "r1", Timeout: time.Minute}

	first, cancelFirst := gateway.NewGatewayContext(t.Context(), route, &gateway.Request{})
	first.Attributes["leftover"] = "value"
	cancelFirst()
	gateway.ReleaseGatewayContext(first)

	second, cancelSecond := gateway.NewGatewayContext(t.Context(), route, &gateway.Request{})
	defer cancelSecond()
	defer gateway.ReleaseGatewayContext(second)

	if len(second.Attributes) != 0 {
		t.Errorf("expected a fresh context to have empty attributes, actual %v", second.Attributes)
	}
}

func TestNewGatewayContext_RouteTimeoutApplies(t *testing.T) {
	route := &gateway.Route{ID: "r1", Timeout: time.Minute}

	ctx, cancel := gateway.NewGatewayContext(t.Context(), route, &gateway.Request{})
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected gateway context to have a deadline")
	}
	if remaining := time.Until(deadline); remaining > time.Minute {
		t.Errorf("expected deadline within the route timeout, remaining %s", remaining)
	}
}
