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

func TestInMemoryRateLimiter_ImplementsRetryAfterRateLimiter(t *testing.T) {
	var limiter ratelimit.RateLimiter = ratelimit.NewInMemoryRateLimiter(&MockTimeProvider{WantedTime: time.Now()}, 1, 1)

	if _, ok := limiter.(ratelimit.RetryAfterRateLimiter); !ok {
		t.Errorf("expected %T to implement ratelimit.RetryAfterRateLimiter", limiter)
	}
}

func TestInMemoryRateLimiter_AllowWithRetryAfter(t *testing.T) {
	tests := []struct {
		name              string
		rate              int
		burst             int
		callsBeforeCheck  int
		idleBeforeCheck   time.Duration
		expectedRemaining int
		expectedWait      time.Duration
		expectedAllow     bool
	}{
		{
			name:              "allow with retry after should return no wait when the request is allowed",
			rate:              1,
			burst:             2,
			callsBeforeCheck:  1,
			expectedAllow:     true,
			expectedRemaining: 0,
			expectedWait:      0,
		},
		{
			name:             "allow with retry after should return the whole refill time when the bucket is empty",
			rate:             1,
			burst:            1,
			callsBeforeCheck: 1,
			expectedWait:     time.Second,
		},
		{
			name:             "allow with retry after should return the time left when the bucket is partly refilled",
			rate:             1,
			burst:            1,
			callsBeforeCheck: 1,
			idleBeforeCheck:  250 * time.Millisecond,
			expectedWait:     750 * time.Millisecond,
		},
		{
			name:             "allow with retry after should return the time for one token when the burst is larger",
			rate:             2,
			burst:            3,
			callsBeforeCheck: 3,
			expectedWait:     500 * time.Millisecond,
		},
		{
			name:             "allow with retry after should return a shorter wait when the rate is higher",
			rate:             10,
			burst:            1,
			callsBeforeCheck: 1,
			idleBeforeCheck:  40 * time.Millisecond,
			expectedWait:     60 * time.Millisecond,
		},
		{
			name:             "allow with retry after should round the wait up when it is not a whole number of nanoseconds",
			rate:             3,
			burst:            1,
			callsBeforeCheck: 1,
			expectedWait:     333_333_334 * time.Nanosecond,
		},
		{
			name:             "allow with retry after should return no wait when the rate is zero and the bucket never refills",
			rate:             0,
			burst:            1,
			callsBeforeCheck: 1,
			idleBeforeCheck:  time.Minute,
			expectedWait:     0,
		},
		{
			name:             "allow with retry after should return no wait when the rate is negative and the bucket never refills",
			rate:             -1,
			burst:            1,
			callsBeforeCheck: 1,
			expectedWait:     0,
		},
		{
			name:             "allow with retry after should return no wait when the burst cannot hold a token",
			rate:             1,
			burst:            0,
			callsBeforeCheck: 0,
			expectedWait:     0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := &MockTimeProvider{WantedTime: time.Now(), Increment: 0}
			limiter := ratelimit.NewInMemoryRateLimiter(clock, tt.rate, tt.burst)
			for range tt.callsBeforeCheck {
				limiter.AllowWithRetryAfter("key")
			}
			clock.WantedTime = clock.WantedTime.Add(tt.idleBeforeCheck)

			allow, remaining, wait := limiter.AllowWithRetryAfter("key")

			if tt.expectedAllow != allow {
				t.Errorf("expected %t actual %t", tt.expectedAllow, allow)
			}
			if tt.expectedRemaining != remaining {
				t.Errorf("expected %d actual %d", tt.expectedRemaining, remaining)
			}
			if tt.expectedWait != wait {
				t.Errorf("expected %s actual %s", tt.expectedWait, wait)
			}
		})
	}
}

// The wait has to be exact in both directions: a client that waits it out is allowed, and
// one that comes back a nanosecond earlier is still rejected.
func TestInMemoryRateLimiter_AllowWithRetryAfter_AllowsExactlyAfterTheWait(t *testing.T) {
	tests := []struct {
		name            string
		rate            int
		burst           int
		idleBeforeCheck time.Duration
	}{
		{name: "wait should be exact when the bucket is empty", rate: 1, burst: 1},
		{name: "wait should be exact when the bucket is partly refilled", rate: 1, burst: 1, idleBeforeCheck: 123_456_789},
		{name: "wait should be exact when it is not a whole number of nanoseconds", rate: 3, burst: 2},
		{name: "wait should be exact when the rate is a prime", rate: 7, burst: 5, idleBeforeCheck: 100 * time.Millisecond},
		{name: "wait should be exact when the division is not exact", rate: 10, burst: 1, idleBeforeCheck: 40 * time.Millisecond},
		{name: "wait should be exact when the rate is high", rate: 1000, burst: 10, idleBeforeCheck: 333 * time.Microsecond},
		{name: "wait should be exact when the rate is very high", rate: 999_983, burst: 3, idleBeforeCheck: 17 * time.Nanosecond},
		// Here the division lands just under a whole number of nanoseconds, so the truncated
		// wait is two nanoseconds short instead of one.
		{name: "wait should be exact when rounding loses more than a nanosecond", rate: 25, burst: 1, idleBeforeCheck: 6900 * time.Microsecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, clock, wait := newRejectingLimiter(t, tt.rate, tt.burst, tt.idleBeforeCheck)
			clock.WantedTime = clock.WantedTime.Add(wait)

			if allow, _, _ := limiter.AllowWithRetryAfter("key"); !allow {
				t.Errorf("expected %t actual %t after waiting %s", true, allow, wait)
			}

			// A second limiter with the same history: the call above moved the bucket of
			// the first one.
			limiter, clock, wait = newRejectingLimiter(t, tt.rate, tt.burst, tt.idleBeforeCheck)
			clock.WantedTime = clock.WantedTime.Add(wait - time.Nanosecond)

			if allow, _, _ := limiter.AllowWithRetryAfter("key"); allow {
				t.Errorf("expected %t actual %t after waiting %s", false, allow, wait-time.Nanosecond)
			}
		})
	}
}

// newRejectingLimiter spends the burst of a new limiter, lets idle pass and returns the
// limiter with its clock and the wait of the request it has just rejected.
func newRejectingLimiter(t *testing.T, rate, burst int, idle time.Duration) (
	*ratelimit.InMemoryRateLimiter, *MockTimeProvider, time.Duration) {
	t.Helper()
	clock := &MockTimeProvider{WantedTime: time.Now(), Increment: 0}
	limiter := ratelimit.NewInMemoryRateLimiter(clock, rate, burst)
	for range burst {
		limiter.AllowWithRetryAfter("key")
	}
	clock.WantedTime = clock.WantedTime.Add(idle)
	allow, _, wait := limiter.AllowWithRetryAfter("key")
	if allow || wait <= 0 {
		t.Fatalf("expected a rejected request with a wait actual %t %s", allow, wait)
	}
	return limiter, clock, wait
}

func TestInMemoryRateLimiter_AllowWithRetryAfter_DoesNotAllocateForExistingKey(t *testing.T) {
	// The clock moves 1ms per call with a rate of one token per second, so almost every
	// call is rejected and goes through the wait computation.
	limiter := ratelimit.NewInMemoryRateLimiter(&MockTimeProvider{WantedTime: time.Now(), Increment: 1}, 1, 2)
	limiter.AllowWithRetryAfter("key")

	allocs := testing.AllocsPerRun(100, func() {
		limiter.AllowWithRetryAfter("key")
	})

	if allocs != 0 {
		t.Errorf("expected %v actual %v", 0, allocs)
	}
}

// The decision and the wait come from the same locked call, so they can never disagree,
// however the calls interleave: with a frozen clock an allowed request has no wait and a
// rejected one always waits for a whole token.
func TestInMemoryRateLimiter_AllowWithRetryAfter_Concurrent(t *testing.T) {
	const (
		goroutines = 8
		calls      = 2000
		rate       = 4
		burst      = 50
	)
	limiter := ratelimit.NewInMemoryRateLimiter(&MockTimeProvider{WantedTime: time.Now(), Increment: 0}, rate, burst)
	var allowed, inconsistent atomic.Int64
	var wg sync.WaitGroup

	for range goroutines {
		wg.Go(func() {
			for range calls {
				ok, _, wait := limiter.AllowWithRetryAfter("key")
				if ok {
					allowed.Add(1)
				}
				if (ok && wait != 0) || (!ok && wait != time.Second/rate) {
					inconsistent.Add(1)
				}
			}
		})
	}
	wg.Wait()

	if allowed.Load() != burst {
		t.Errorf("expected %d actual %d", burst, allowed.Load())
	}
	if inconsistent.Load() != 0 {
		t.Errorf("expected %d actual %d", 0, inconsistent.Load())
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
