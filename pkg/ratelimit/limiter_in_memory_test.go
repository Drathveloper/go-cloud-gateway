package ratelimit_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/ratelimit"
)

func TestInMemoryRateLimiter_Allow(t *testing.T) {
	tests := []struct {
		timeProvider      common.TimeProvider
		getKey            func() string
		name              string
		timesBeforeCheck  int
		rate              int
		burst             int
		expectedRemaining int
		expectedAllow     bool
	}{
		{
			name: "test allow should return true when bucket is not full",
			timeProvider: &MockTimeProvider{
				WantedTime: time.Now(),
				Increment:  0,
			},
			timesBeforeCheck: 0,
			rate:             1,
			burst:            2,
			getKey: func() string {
				return "key"
			},
			expectedAllow:     true,
			expectedRemaining: 1,
		},
		{
			name: "test allow should return false when bucket is full",
			timeProvider: &MockTimeProvider{
				WantedTime: time.Now(),
				Increment:  0,
			},
			timesBeforeCheck: 2,
			rate:             1,
			burst:            2,
			getKey: func() string {
				return "key"
			},
			expectedAllow:     false,
			expectedRemaining: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := ratelimit.NewInMemoryRateLimiter(tt.timeProvider, tt.rate, tt.burst)

			for range tt.timesBeforeCheck {
				_, _ = limiter.Allow(tt.getKey())
			}

			allow, remaining := limiter.Allow(tt.getKey())

			if tt.expectedRemaining != remaining {
				t.Errorf("expected %d actual %d", tt.expectedRemaining, remaining)
			}
			if tt.expectedAllow != allow {
				t.Errorf("expected %t actual %t", tt.expectedAllow, allow)
			}
		})
	}
}

func TestNewInMemoryRateLimiterBuilder(t *testing.T) {
	tests := []struct {
		args        map[string]any
		expectedErr error
		name        string
	}{
		{
			name: "new in memory rate limiter builder should succeed when args are valid",
			args: map[string]any{
				"rate":  1,
				"burst": 2,
			},
			expectedErr: nil,
		},
		{
			name: "new in memory rate limiter builder should return error when rate is not valid",
			args: map[string]any{
				"rate":  "potato",
				"burst": 2,
			},
			expectedErr: errors.New("failed to convert 'rate' attribute: value is required to be a valid int"),
		},
		{
			name: "new in memory rate limiter builder should return error when burst is not valid",
			args: map[string]any{
				"rate":  1,
				"burst": "potato",
			},
			expectedErr: errors.New("failed to convert 'burst' attribute: value is required to be a valid int"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := ratelimit.NewInMemoryRateLimiterBuilder()

			_, err := builder.Build(tt.args)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}
