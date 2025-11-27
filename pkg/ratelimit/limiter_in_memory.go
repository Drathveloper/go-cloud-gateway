package ratelimit

import (
	"fmt"
	"sync"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

// InMemoryRateLimiterName is the registry name of the in memory rate limiter.
const InMemoryRateLimiterName = "in-memory"

// InMemoryRateLimiter is an in memory rate limiter.
type InMemoryRateLimiter struct {
	time    shared.TimeProvider
	buckets map[string]*TokenBucket
	rate    int
	burst   int
	mutex   sync.Mutex
}

// NewInMemoryRateLimiter creates a new in memory rate limiter.
func NewInMemoryRateLimiter(time shared.TimeProvider, rate, burst int) *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		time:    time,
		buckets: make(map[string]*TokenBucket),
		rate:    rate,
		burst:   burst,
	}
}

// NewInMemoryRateLimiterBuilder creates a new in memory rate limiter builder.
func NewInMemoryRateLimiterBuilder() RateLimiterBuilderFunc {
	return func(args map[string]any) (RateLimiter, error) {
		rate, err := shared.ConvertToInt(args["rate"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'rate' attribute: %w", err)
		}
		burst, err := shared.ConvertToInt(args["burst"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'burst' attribute: %w", err)
		}
		return NewInMemoryRateLimiter(&shared.RealTime{}, rate, burst), nil
	}
}

// Allow check if the given key is allowed to pass based on the in-memory rate limiter. Returns true if allowed,
// false if not allowed, and the remaining tokens.
func (rl *InMemoryRateLimiter) Allow(key string) (bool, int) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	bucket, ok := rl.buckets[key]
	if !ok {
		bucket = NewTokenBucket(rl.time, rl.rate, rl.burst)
		rl.buckets[key] = bucket
	}
	return bucket.Allow()
}
