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
