package ratelimit

import (
	"strconv"
	"testing"
	"time"
)

// fakeClock is a time provider that only moves when the test says so.
type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func TestInMemoryRateLimiter_Allow_EvictsFullBuckets(t *testing.T) {
	tests := []struct {
		name            string
		keys            []string
		callsPerKey     int
		rate            int
		burst           int
		idle            time.Duration
		expectedBuckets int
	}{
		{
			name:            "allow should evict buckets when they have refilled and the sweep interval elapsed",
			keys:            []string{"a", "b", "c"},
			callsPerKey:     1,
			rate:            1,
			burst:           2,
			idle:            time.Second,
			expectedBuckets: 1,
		},
		{
			name:            "allow should evict buckets when they have been idle for much longer than the refill time",
			keys:            []string{"a", "b", "c"},
			callsPerKey:     2,
			rate:            1,
			burst:           2,
			idle:            time.Hour,
			expectedBuckets: 1,
		},
		{
			name:            "allow should keep buckets when the sweep interval has not elapsed",
			keys:            []string{"a", "b", "c"},
			callsPerKey:     1,
			rate:            10,
			burst:           2,
			idle:            sweepInterval - time.Millisecond,
			expectedBuckets: 4,
		},
		{
			name:            "allow should keep buckets when they are still refilling",
			keys:            []string{"a", "b", "c"},
			callsPerKey:     3,
			rate:            1,
			burst:           5,
			idle:            2 * time.Second,
			expectedBuckets: 4,
		},
		{
			name:            "allow should keep buckets when they are one instant short of full",
			keys:            []string{"a", "b", "c"},
			callsPerKey:     5,
			rate:            1,
			burst:           5,
			idle:            5*time.Second - time.Nanosecond,
			expectedBuckets: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := &fakeClock{now: time.Now()}
			limiter := NewInMemoryRateLimiter(clock, tt.rate, tt.burst)
			for _, key := range tt.keys {
				for range tt.callsPerKey {
					_, _ = limiter.Allow(key)
				}
			}

			clock.advance(tt.idle)
			_, _ = limiter.Allow("trigger")

			if tt.expectedBuckets != len(limiter.buckets) {
				t.Errorf("expected %d actual %d", tt.expectedBuckets, len(limiter.buckets))
			}
		})
	}
}

func TestInMemoryRateLimiter_Allow_StaysBoundedWithDistinctKeys(t *testing.T) {
	const calls = 100_000
	// One call per millisecond with a rate of 10: a bucket is full again 100 calls after
	// its only use, and a sweep runs every 1000 calls.
	clock := &fakeClock{now: time.Now()}
	limiter := NewInMemoryRateLimiter(clock, 10, 5)
	maxBuckets := 0

	for i := range calls {
		clock.advance(time.Millisecond)
		_, _ = limiter.Allow(strconv.Itoa(i))
		maxBuckets = max(maxBuckets, len(limiter.buckets))
	}

	// Between two sweeps at most 1000 keys are added to the 100 that were still refilling.
	if maxBuckets > 1100 {
		t.Errorf("expected at most %d buckets actual %d", 1100, maxBuckets)
	}
}

// Deleting a bucket must never change a decision: the limiter has to answer exactly like
// one bucket per key that is never deleted.
func TestInMemoryRateLimiter_Allow_SweepDoesNotChangeDecisions(t *testing.T) {
	tests := []struct {
		name  string
		rate  int
		burst int
		keys  int
	}{
		{name: "decisions should match buckets that are never evicted when the refill is fast", rate: 50, burst: 3, keys: 5},
		{name: "decisions should match buckets that are never evicted when the refill is slow", rate: 1, burst: 4, keys: 5},
		{name: "decisions should match buckets that are never evicted when the burst is large", rate: 20, burst: 40, keys: 3},
		{name: "decisions should match buckets that are never evicted when there are many keys", rate: 3, burst: 2, keys: 97},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := &fakeClock{now: time.Now()}
			limiter := NewInMemoryRateLimiter(clock, tt.rate, tt.burst)
			reference := make(map[string]*TokenBucket)
			evicted := false

			for i := range 20_000 {
				// A fixed, irregular schedule: bursts of calls a few milliseconds apart,
				// separated by pauses long enough for sweeps to find full buckets.
				step := time.Duration(i*37%11) * time.Millisecond
				if i%257 == 0 {
					step = time.Duration(i%7) * time.Second
				}
				clock.advance(step)
				key := strconv.Itoa(i * 31 % tt.keys)
				if _, ok := reference[key]; !ok {
					reference[key] = newTokenBucketAt(clock, tt.rate, tt.burst, clock.now)
				}
				expectedAllow, expectedRemaining := reference[key].allowAt(clock.now)

				allow, remaining := limiter.Allow(key)

				if expectedAllow != allow || expectedRemaining != remaining {
					t.Fatalf("call %d key %s: expected %t %d actual %t %d",
						i, key, expectedAllow, expectedRemaining, allow, remaining)
				}
				evicted = evicted || len(limiter.buckets) < len(reference)
			}

			if !evicted {
				t.Error("expected the schedule to evict at least one bucket")
			}
		})
	}
}
