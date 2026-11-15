package filter_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/ratelimit"
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

// MockRetryAfterLimiter is a limiter that can also report how long a rejected key has to
// wait. It counts the calls to each method: the filter has to take the decision and the
// wait from a single call.
type MockRetryAfterLimiter struct {
	AllowCalls               int
	AllowWithRetryAfterCalls int
	Remaining                int
	RetryAfter               time.Duration
	Allowed                  bool
}

func (m *MockRetryAfterLimiter) Allow(_ string) (bool, int) {
	m.AllowCalls++
	return m.Allowed, m.Remaining
}

func (m *MockRetryAfterLimiter) AllowWithRetryAfter(_ string) (bool, int, time.Duration) {
	m.AllowWithRetryAfterCalls++
	return m.Allowed, m.Remaining, m.RetryAfter
}

// frozenClock is a time provider that never moves, so a bucket never refills.
type frozenClock struct {
	now time.Time
}

func (c *frozenClock) Now() time.Time {
	return c.now
}

func TestRateLimit_PreProcess_RetryAfter(t *testing.T) {
	tests := []struct {
		limiter            ratelimit.RateLimiter
		name               string
		expectedMsg        string
		calls              int
		expectedRetryAfter time.Duration
		expectedRemaining  int
		expectedRejected   bool
	}{
		{
			name:             "pre process should succeed when a limiter that reports the wait allows the request",
			limiter:          &MockRetryAfterLimiter{Allowed: true, Remaining: 3, RetryAfter: time.Second},
			calls:            1,
			expectedRejected: false,
		},
		{
			name:               "pre process should return the wait when the limiter reports it",
			limiter:            &MockRetryAfterLimiter{Allowed: false, Remaining: 0, RetryAfter: 1500 * time.Millisecond},
			calls:              1,
			expectedRejected:   true,
			expectedRetryAfter: 1500 * time.Millisecond,
			expectedMsg:        "rate limit exceeded: remaining 0, retry after 1.5s",
		},
		{
			name:               "pre process should return no wait when the limiter reports zero",
			limiter:            &MockRetryAfterLimiter{Allowed: false, Remaining: 2, RetryAfter: 0},
			calls:              1,
			expectedRejected:   true,
			expectedRetryAfter: 0,
			expectedRemaining:  2,
			expectedMsg:        "rate limit exceeded: remaining 2",
		},
		{
			name:               "pre process should return no wait when the limiter reports a negative wait",
			limiter:            &MockRetryAfterLimiter{Allowed: false, Remaining: 0, RetryAfter: -time.Second},
			calls:              1,
			expectedRejected:   true,
			expectedRetryAfter: 0,
			expectedMsg:        "rate limit exceeded: remaining 0",
		},
		{
			name:               "pre process should return no wait when the limiter cannot report it",
			limiter:            &MockLimiter{ExpectedKey: "key", ExpectedRemaining: 0, ExpectedAllow: false, testing: t},
			calls:              1,
			expectedRejected:   true,
			expectedRetryAfter: 0,
			expectedMsg:        "rate limit exceeded: remaining 0",
		},
		{
			name:             "pre process should succeed when the in memory limiter still has tokens",
			limiter:          ratelimit.NewInMemoryRateLimiter(&frozenClock{now: time.Now()}, 1, 1),
			calls:            1,
			expectedRejected: false,
		},
		{
			name:               "pre process should return the wait when the in memory limiter has spent its burst",
			limiter:            ratelimit.NewInMemoryRateLimiter(&frozenClock{now: time.Now()}, 1, 1),
			calls:              2,
			expectedRejected:   true,
			expectedRetryAfter: time.Second,
			expectedMsg:        "rate limit exceeded: remaining 0, retry after 1s",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gateway.NewGatewayContext(t.Context(), &gateway.Route{}, &gateway.Request{})
			f := filter.NewRateLimitFilter(tt.limiter, func(_ *gateway.Context) string {
				return "key"
			})

			var err error
			for range tt.calls {
				err = f.PreProcess(ctx)
			}

			if !tt.expectedRejected {
				if err != nil {
					t.Fatalf("expected err %v actual %v", nil, err)
				}
				return
			}
			if !errors.Is(err, filter.ErrRateLimitExceeded) {
				t.Fatalf("expected err %v actual %v", filter.ErrRateLimitExceeded, err)
			}
			if tt.expectedMsg != err.Error() {
				t.Errorf("expected err message %s actual %s", tt.expectedMsg, err)
			}
			rateLimitErr, ok := errors.AsType[*filter.RateLimitExceededError](err)
			if !ok {
				t.Fatalf("expected err of type %T actual %T", rateLimitErr, err)
			}
			if tt.expectedRetryAfter != rateLimitErr.RetryAfter {
				t.Errorf("expected retry after %s actual %s", tt.expectedRetryAfter, rateLimitErr.RetryAfter)
			}
			if tt.expectedRemaining != rateLimitErr.Remaining {
				t.Errorf("expected remaining %d actual %d", tt.expectedRemaining, rateLimitErr.Remaining)
			}
		})
	}
}

// A second lookup after Allow would describe the limiter after other requests changed
// it, so the filter has to ask a limiter that reports the wait exactly once.
func TestRateLimit_PreProcess_AsksTheLimiterOnce(t *testing.T) {
	tests := []struct {
		limiter *MockRetryAfterLimiter
		name    string
	}{
		{
			name:    "pre process should ask the limiter once when the request is allowed",
			limiter: &MockRetryAfterLimiter{Allowed: true},
		},
		{
			name:    "pre process should ask the limiter once when the request is rejected",
			limiter: &MockRetryAfterLimiter{Allowed: false, RetryAfter: time.Second},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gateway.NewGatewayContext(t.Context(), &gateway.Route{}, &gateway.Request{})
			f := filter.NewRateLimitFilter(tt.limiter, func(_ *gateway.Context) string {
				return "key"
			})

			_ = f.PreProcess(ctx)

			if tt.limiter.AllowWithRetryAfterCalls != 1 {
				t.Errorf("expected %d actual %d", 1, tt.limiter.AllowWithRetryAfterCalls)
			}
			if tt.limiter.AllowCalls != 0 {
				t.Errorf("expected %d actual %d", 0, tt.limiter.AllowCalls)
			}
		})
	}
}

func TestRateLimitExceededError(t *testing.T) {
	tests := []struct {
		err         *filter.RateLimitExceededError
		name        string
		expectedMsg string
	}{
		{
			name:        "error should not mention the wait when it is not known",
			err:         &filter.RateLimitExceededError{Remaining: 4},
			expectedMsg: "rate limit exceeded: remaining 4",
		},
		{
			name:        "error should mention the wait when it is known",
			err:         &filter.RateLimitExceededError{Remaining: 0, RetryAfter: 250 * time.Millisecond},
			expectedMsg: "rate limit exceeded: remaining 0, retry after 250ms",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The gateway wraps the filter error twice before the error handler sees it.
			wrapped := fmt.Errorf("gateway request failed: %w", fmt.Errorf("filter failed: %w", tt.err))

			if tt.expectedMsg != tt.err.Error() {
				t.Errorf("expected err message %s actual %s", tt.expectedMsg, tt.err.Error())
			}
			if !errors.Is(wrapped, filter.ErrRateLimitExceeded) {
				t.Errorf("expected err %v actual %v", filter.ErrRateLimitExceeded, wrapped)
			}
			actual, ok := errors.AsType[*filter.RateLimitExceededError](wrapped)
			if !ok || actual != tt.err {
				t.Errorf("expected %v actual %v", tt.err, actual)
			}
		})
	}
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
			ctx, _ := gateway.NewGatewayContext(t.Context(), &gateway.Route{}, &gateway.Request{})
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
