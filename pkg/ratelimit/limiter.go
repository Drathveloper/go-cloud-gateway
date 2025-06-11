package ratelimit

import (
	"sync"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
)

type RateLimiter interface {
	Allow(key string) (bool, int)
}

type InMemoryRateLimiter struct {
	time    common.TimeProvider
	buckets map[string]*TokenBucket
	rate    int
	burst   int
	mutex   sync.Mutex
}

func NewInMemoryRateLimiter(time common.TimeProvider, rate, burst int) *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		time:    time,
		buckets: make(map[string]*TokenBucket),
		rate:    rate,
		burst:   burst,
	}
}

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
