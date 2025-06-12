package filter_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

type MockLimiter struct {
	testing           *testing.T
	ExpectedKey       string
	ExpectedRemaining int
	ExpectedAllow     bool
}

func (m *MockLimiter) Allow(key string) (bool, int) {
	if m.ExpectedKey != key {
		m.testing.Errorf("unexpected call to Allow")
	}
	return m.ExpectedAllow, m.ExpectedRemaining
}

func TestRateLimit_PreProcess(t *testing.T) {
	tests := []struct {
		expectedErr error
		limiter     *MockLimiter
		keyFunc     func(ctx *gateway.Context) string
		name        string
	}{
		{
			name: "pre process should succeed when allow is true",
			limiter: &MockLimiter{
				ExpectedKey:       "key",
				ExpectedRemaining: 1,
				ExpectedAllow:     true,
				testing:           t,
			},
			keyFunc: func(_ *gateway.Context) string {
				return "key"
			},
			expectedErr: nil,
		},
		{
			name: "pre process should return error when allow is false",
			limiter: &MockLimiter{
				ExpectedKey:       "key",
				ExpectedRemaining: 0,
				ExpectedAllow:     false,
				testing:           t,
			},
			keyFunc: func(_ *gateway.Context) string {
				return "key"
			},
			expectedErr: errors.New("rate limit exceeded: remaining 0"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, &gateway.Request{})
			f := filter.NewRateLimitFilter(tt.limiter, tt.keyFunc)

			err := f.PreProcess(ctx)

			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestRateLimit_PostProcess(t *testing.T) {
	f := filter.NewRateLimitFilter(nil, nil)
	if err := f.PostProcess(nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRateLimit_Name(t *testing.T) {
	f := filter.NewRateLimitFilter(nil, nil)
	if f.Name() != filter.RateLimitFilterName {
		t.Errorf("expected name to be %s, got %s", filter.RateLimitFilterName, f.Name())
	}
}

func TestNewRateLimitBuilder(t *testing.T) {
	tests := []struct {
		args        map[string]any
		expectedErr error
		name        string
	}{
		{
			name: "build should succeed when args are present and are valid",
			args: map[string]any{
				"type":  "in-memory",
				"key":   "ip",
				"rate":  1,
				"burst": 1,
			},
			expectedErr: nil,
		},
		{
			name: "build should return error when type is not present",
			args: map[string]any{
				"key":   "ip",
				"rate":  1,
				"burst": 1,
			},
			expectedErr: errors.New("failed to convert 'type' attribute: value is required"),
		},
		{
			name: "build should return error when key is not present",
			args: map[string]any{
				"type":  "in-memory",
				"rate":  1,
				"burst": 1,
			},
			expectedErr: errors.New("failed to convert 'key' attribute: value is required"),
		},
		{
			name: "build should return error when type is not valid",
			args: map[string]any{
				"type":  "other",
				"key":   "ip",
				"rate":  1,
				"burst": 1,
			},
			expectedErr: errors.New("invalid rate limit type: other"),
		},
		{
			name: "build should return error when key is not valid",
			args: map[string]any{
				"type":  "in-memory",
				"key":   "other",
				"rate":  1,
				"burst": 1,
			},
			expectedErr: errors.New("invalid rate limit key: other"),
		},
		{
			name: "build should return error when build key func failed",
			args: map[string]any{
				"type":  "in-memory",
				"key":   "header",
				"rate":  1,
				"burst": 1,
			},
			expectedErr: errors.New("failed to build rate limit key: failed to convert 'header-name' attribute: value is required"),
		},
		{
			name: "build should return error when build limiter failed",
			args: map[string]any{
				"type": "in-memory",
				"key":  "ip",
				"rate": 1,
			},
			expectedErr: errors.New("failed to build rate limiter: failed to convert 'burst' attribute: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := filter.NewRateLimitBuilder().Build(tt.args)
			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}
