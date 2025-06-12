package ratelimit_test

import (
	"net/url"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/ratelimit"
)

func TestNewIPKeyFuncBuilder(t *testing.T) {
	builder := ratelimit.NewIPKeyFuncBuilder()

	keyFunc, err := builder.Build(nil)

	if err != nil {
		t.Error(err)
	}
	if keyFunc == nil {
		t.Error("key func is nil")
	}

	ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, &gateway.Request{
		RemoteAddr: "127.0.0.1",
	})
	key := keyFunc(ctx)

	if key != "127.0.0.1" {
		t.Error("key is not 127.0.0.1")
	}
}

func TestNewPathKeyFuncBuilder(t *testing.T) {
	builder := ratelimit.NewPathKeyFuncBuilder()

	keyFunc, err := builder.Build(nil)

	if err != nil {
		t.Error(err)
	}
	if keyFunc == nil {
		t.Error("key func is nil")
	}

	ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, &gateway.Request{
		URL: &url.URL{
			Path: "/test",
		},
	})
	key := keyFunc(ctx)

	if key != "/test" {
		t.Error("key is not /test")
	}
}

func TestNewPathAndMethodKeyFuncBuilder(t *testing.T) {
	builder := ratelimit.NewPathAndMethodKeyFuncBuilder()

	keyFunc, err := builder.Build(nil)

	if err != nil {
		t.Error(err)
	}
	if keyFunc == nil {
		t.Error("key func is nil")
	}

	ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, &gateway.Request{
		URL: &url.URL{
			Path: "/test",
		},
		Method: "GET",
	})
	key := keyFunc(ctx)

	if key != "GET/test" {
		t.Error("key is not GET/test")
	}
}

func TestNewHeaderKeyFuncBuilder_ShouldSucceed(t *testing.T) {
	builder := ratelimit.NewHeaderKeyFuncBuilder()

	keyFunc, err := builder.Build(map[string]any{
		"header-name": "X-Forwarded-For",
	})

	if err != nil {
		t.Error(err)
	}
	if keyFunc == nil {
		t.Error("key func is nil")
	}

	ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, &gateway.Request{
		Headers: map[string][]string{"X-Forwarded-For": {"127.0.0.1"}},
	})
	key := keyFunc(ctx)

	if key != "127.0.0.1" {
		t.Error("key is not 127.0.0.1")
	}
}

func TestNewHeaderKeyFuncBuilder_ShouldReturnErrorWhenHeaderNameNotPresent(t *testing.T) {
	expectedErr := "failed to convert 'header-name' attribute: value is required"
	builder := ratelimit.NewHeaderKeyFuncBuilder()

	keyFunc, err := builder.Build(map[string]any{})

	if keyFunc != nil {
		t.Error("key func must be nil")
	}
	if err == nil {
		t.Error("expected error but got nil")
	}
	if err != nil && err.Error() != expectedErr {
		t.Errorf("expected %s got %s", expectedErr, err.Error())
	}
}

func TestNewQueryKeyFuncBuilder_ShouldSucceed(t *testing.T) {
	builder := ratelimit.NewQueryKeyFuncBuilder()

	keyFunc, err := builder.Build(map[string]any{
		"query-param": "test",
	})

	if err != nil {
		t.Error(err)
	}
	if keyFunc == nil {
		t.Error("key func is nil")
	}

	ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, &gateway.Request{
		URL: &url.URL{
			RawQuery: "test=abc",
		},
	})
	key := keyFunc(ctx)

	if key != "abc" {
		t.Error("key is not abc")
	}
}

func TestNewQueryKeyFuncBuilder_ShouldReturnErrorWhenHeaderNameNotPresent(t *testing.T) {
	expectedErr := "failed to convert 'query-param' attribute: value is required"
	builder := ratelimit.NewQueryKeyFuncBuilder()

	keyFunc, err := builder.Build(map[string]any{})

	if keyFunc != nil {
		t.Error("key func must be nil")
	}
	if err == nil {
		t.Error("expected error but got nil")
	}
	if err != nil && err.Error() != expectedErr {
		t.Errorf("expected %s got %s", expectedErr, err.Error())
	}
}
