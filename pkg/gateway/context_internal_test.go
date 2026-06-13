package gateway

import (
	"testing"
	"time"
)

func TestReleaseGatewayContext_DropsReferences(t *testing.T) {
	route := &Route{ID: "r1", Timeout: time.Minute}
	ctx, cancel := NewGatewayContext(t.Context(), route, &Request{})
	defer cancel()
	ctx.Response = &Response{}
	ctx.Attributes["key"] = "value"

	ReleaseGatewayContext(ctx)

	if ctx.Request != nil || ctx.Response != nil || ctx.Route != nil || ctx.Logger != nil || ctx.Context != nil {
		t.Error("expected all references dropped after release")
	}
	if len(ctx.Attributes) != 0 {
		t.Errorf("expected attributes cleared after release, actual %v", ctx.Attributes)
	}
}
