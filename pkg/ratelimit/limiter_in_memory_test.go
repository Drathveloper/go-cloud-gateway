package ratelimit_test

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/ratelimit"
)

func TestInMemoryRateLimiter_Allow(t *testing.T) {
	tests := []struct {
		timeProvider      shared.TimeProvider
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
		{
			// The clock moves 600ms per call, so a sweep runs on the third call, while
			// the bucket is still refilling. Evicting it there would hand out a fresh burst.
			name: "test allow should return false when a sweep runs while the bucket is refilling",
			timeProvider: &MockTimeProvider{
				WantedTime: time.Now(),
				Increment:  600,
			},
			timesBeforeCheck: 3,
			rate:             1,
			burst:            2,
			getKey: func() string {
				return "key"
			},
			expectedAllow:     false,
			expectedRemaining: 0,
		},
		{
			name: "test allow should return true with a full burst when the key has been idle long enough to refill",
			timeProvider: &MockTimeProvider{
				WantedTime: time.Now(),
				Increment:  5000,
			},
			timesBeforeCheck: 4,
			rate:             1,
			burst:            3,
			getKey: func() string {
				return "key"
			},
			expectedAllow:     true,
			expectedRemaining: 2,
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

func TestInMemoryRateLimiter_Allow_DoesNotAllocateForExistingKey(t *testing.T) {
	limiter := ratelimit.NewInMemoryRateLimiter(&MockTimeProvider{WantedTime: time.Now(), Increment: 1}, 1, 2)
	_, _ = limiter.Allow("key")

	allocs := testing.AllocsPerRun(100, func() {
		_, _ = limiter.Allow("key")
	})

	if allocs != 0 {
		t.Errorf("expected %v actual %v", 0, allocs)
	}
}

func TestInMemoryRateLimiter_Allow_Concurrent(t *testing.T) {
	const (
		goroutines = 8
		calls      = 2000
		keys       = 4
		rate       = 1
		burst      = 5
	)
	// The limiter reads the clock under its mutex, so the mock needs no locking of its
	// own. It moves 1ms per call: the run covers 16s and many sweeps.
	limiter := ratelimit.NewInMemoryRateLimiter(&MockTimeProvider{WantedTime: time.Now(), Increment: 1}, rate, burst)
	allowed := make([]atomic.Int64, keys)
	var wg sync.WaitGroup

	for worker := range goroutines {
		wg.Go(func() {
			for call := range calls {
				key := (worker + call) % keys
				if ok, _ := limiter.Allow(strconv.Itoa(key)); ok {
					allowed[key].Add(1)
				}
			}
		})
	}
	wg.Wait()

	// A key can never get more than its burst plus what the rate refills during the run.
	maxAllowed := int64(burst + rate*goroutines*calls/1000)
	for key := range allowed {
		actual := allowed[key].Load()
		if actual < burst || actual > maxAllowed {
			t.Errorf("key %d: expected between %d and %d actual %d", key, burst, maxAllowed, actual)
		}
	}
}

func TestNewInMemoryRateLimiterBuilder(t *testing.T) {
	tests := []struct {
		args          map[string]any
		expectedErr   error
		expectedCause error
		name          string
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
		{
			name: "new in memory rate limiter builder should return error when rate is zero",
			args: map[string]any{
				"rate":  0,
				"burst": 2,
			},
			expectedErr:   errors.New("failed to validate 'rate' attribute: value is required to be greater than zero: 0"),
			expectedCause: ratelimit.ErrNonPositiveRateLimitValue,
		},
		{
			name: "new in memory rate limiter builder should return error when rate is negative",
			args: map[string]any{
				"rate":  "-3",
				"burst": 2,
			},
			expectedErr:   errors.New("failed to validate 'rate' attribute: value is required to be greater than zero: -3"),
			expectedCause: ratelimit.ErrNonPositiveRateLimitValue,
		},
		{
			name: "new in memory rate limiter builder should return error when rate is a fraction truncated to zero",
			args: map[string]any{
				"rate":  0.5,
				"burst": 2,
			},
			expectedErr:   errors.New("failed to validate 'rate' attribute: value is required to be greater than zero: 0"),
			expectedCause: ratelimit.ErrNonPositiveRateLimitValue,
		},
		{
			name: "new in memory rate limiter builder should return error when burst is zero",
			args: map[string]any{
				"rate":  1,
				"burst": 0,
			},
			expectedErr:   errors.New("failed to validate 'burst' attribute: value is required to be greater than zero: 0"),
			expectedCause: ratelimit.ErrNonPositiveRateLimitValue,
		},
		{
			name: "new in memory rate limiter builder should return error when burst is negative",
			args: map[string]any{
				"rate":  1,
				"burst": -1,
			},
			expectedErr:   errors.New("failed to validate 'burst' attribute: value is required to be greater than zero: -1"),
			expectedCause: ratelimit.ErrNonPositiveRateLimitValue,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := ratelimit.NewInMemoryRateLimiterBuilder()

			limiter, err := builder.Build(tt.args)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if tt.expectedCause != nil && !errors.Is(err, tt.expectedCause) {
				t.Errorf("expected err %v to wrap %v", err, tt.expectedCause)
			}
			if tt.expectedErr != nil && limiter != nil {
				t.Errorf("expected %v actual %v", nil, limiter)
			}
		})
	}
}
