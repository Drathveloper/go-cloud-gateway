package ratelimit

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

// InMemoryRateLimiterName is the registry name of the in memory rate limiter.
const InMemoryRateLimiterName = "in-memory"

// sweepInterval is the minimum time between two sweeps of the buckets.
//
// A sweep visits every bucket, so its cost has to be covered by the calls made between
// two sweeps. A bucket survives a sweep only while it is not full, and each token taken
// from it keeps it short of full for 1/rate seconds. Over any run of calls the sweeps
// therefore make at most 2 + 1/(rate*sweepInterval) bucket visits per call: one when a
// sweep finds the bucket still refilling, one when a sweep deletes it, and the rest for
// the time the taken token keeps it short of full. With one second and a rate of at
// least one token per second that is at most three visits per call, whatever the burst
// and however many keys are in use. A shorter interval would free memory sooner but
// visit the buckets that are still refilling more often; a longer one would let a client
// that forges its key pile up more buckets before they are deleted.
//
// It also bounds the memory: a key is kept for at most sweepInterval after its bucket has
// refilled, instead of forever.
const sweepInterval = time.Second

// ErrNonPositiveRateLimitValue is returned when the rate or the burst of a rate limiter is
// zero or negative: such a limiter would reject every request.
var ErrNonPositiveRateLimitValue = errors.New("value is required to be greater than zero")

// InMemoryRateLimiter is an in memory rate limiter.
type InMemoryRateLimiter struct {
	time      shared.TimeProvider
	buckets   map[string]*TokenBucket
	lastSweep time.Time
	rate      int
	burst     int
	mutex     sync.Mutex
}

// NewInMemoryRateLimiter creates a new in memory rate limiter.
//
// The rate and the burst are expected to be greater than zero, but they are not checked
// here. With a rate of zero or less the buckets never refill: once their burst is spent
// they reject every request, and they are never deleted.
func NewInMemoryRateLimiter(time shared.TimeProvider, rate, burst int) *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		time:    time,
		buckets: make(map[string]*TokenBucket),
		rate:    rate,
		burst:   burst,
	}
}

// NewInMemoryRateLimiterBuilder creates a new in memory rate limiter builder.
//
// The "rate" argument is the number of tokens added per second and the "burst" argument
// is the bucket size. Both are required to be greater than zero.
func NewInMemoryRateLimiterBuilder() RateLimiterBuilderFunc {
	return func(args map[string]any) (RateLimiter, error) {
		rate, err := convertToPositiveInt(args, "rate")
		if err != nil {
			return nil, err
		}
		burst, err := convertToPositiveInt(args, "burst")
		if err != nil {
			return nil, err
		}
		return NewInMemoryRateLimiter(&shared.RealTime{}, rate, burst), nil
	}
}

// Allow check if the given key is allowed to pass based on the in-memory rate limiter. Returns true if allowed,
// false if not allowed, and the remaining tokens.
func (rl *InMemoryRateLimiter) Allow(key string) (bool, int) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	// One clock reading serves the sweep check, the bucket creation and the bucket itself.
	now := rl.time.Now()
	if now.Sub(rl.lastSweep) >= sweepInterval {
		rl.sweep(now)
	}
	bucket, ok := rl.buckets[key]
	if !ok {
		bucket = newTokenBucketAt(rl.time, rl.rate, rl.burst, now)
		rl.buckets[key] = bucket
	}
	return bucket.allowAt(now)
}

// sweep deletes the buckets that are full at now. The keys can be chosen by the client
// (the ip key comes from X-Forwarded-For), so without it the map would grow without bound.
//
// A full bucket is indistinguishable from the one Allow creates for an unknown key, so
// deleting it never changes a decision. It runs from Allow rather than from a goroutine,
// which leaves the limiter with nothing to stop or close. The caller holds the mutex.
func (rl *InMemoryRateLimiter) sweep(now time.Time) {
	rl.lastSweep = now
	for key, bucket := range rl.buckets {
		if bucket.isFullAt(now) {
			delete(rl.buckets, key)
		}
	}
}

// convertToPositiveInt reads the named argument as an int greater than zero.
func convertToPositiveInt(args map[string]any, name string) (int, error) {
	value, err := shared.ConvertToInt(args[name])
	if err != nil {
		return 0, fmt.Errorf("failed to convert '%s' attribute: %w", name, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("failed to validate '%s' attribute: %w: %d", name, ErrNonPositiveRateLimitValue, value)
	}
	return value, nil
}
