package ratelimit_test

import (
	"io"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/ratelimit"
)

func TestKeyFuncRegistry_Register(t *testing.T) {
	registry := make(ratelimit.KeyFuncRegistry)

	registry.Register("potato", ratelimit.KeyFuncBuilderFunc(func(_ map[string]any) (ratelimit.KeyFunc, error) {
		return nil, io.EOF
	}))

	if len(registry) != 1 {
		t.Error("expected 1 rate limiter but got", len(registry))
	}
}

func TestRateLimiterRegistry_Register(t *testing.T) {
	registry := make(ratelimit.RateLimiterRegistry)

	registry.Register("potato", ratelimit.RateLimiterBuilderFunc(func(_ map[string]any) (ratelimit.RateLimiter, error) {
		return nil, io.EOF
	}))

	if len(registry) != 1 {
		t.Error("expected 1 rate limiter but got", len(registry))
	}
}
