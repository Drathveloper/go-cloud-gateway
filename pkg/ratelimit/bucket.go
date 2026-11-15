package ratelimit

import (
	"sync"
	"time"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

// TokenBucket is a token bucket. It is safe for concurrent use.
type TokenBucket struct {
	time shared.TimeProvider
	// lastUpdate is held by value: a pointer would force a heap allocation on every Allow.
	lastUpdate time.Time
	tokens     float64
	rate       int
	burst      int
	mutex      sync.Mutex
}

// NewTokenBucket creates a new token bucket.
//
// The token bucket will allow the given rate of tokens per second.
// The token bucket will allow the given burst of tokens.
func NewTokenBucket(time shared.TimeProvider, rate, burst int) *TokenBucket {
	return newTokenBucketAt(time, rate, burst, time.Now())
}

// newTokenBucketAt creates a token bucket whose refill starts at now. It lets a caller
// that already read the clock create the bucket and take from it at the same instant.
func newTokenBucketAt(time shared.TimeProvider, rate, burst int, now time.Time) *TokenBucket {
	return &TokenBucket{
		time:       time,
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastUpdate: now,
	}
}

// Allow checks if the token bucket allows the request.
//
// If the token bucket allows the request, Allow returns true and the remaining tokens.
// If the token bucket does not allow the request, Allow returns false and the remaining tokens.
func (tb *TokenBucket) Allow() (bool, int) {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	// The clock is read under the lock so concurrent callers apply their readings in order.
	return tb.take(tb.time.Now())
}

// allowAt is Allow for a caller that already read the clock.
func (tb *TokenBucket) allowAt(now time.Time) (bool, int) {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	return tb.take(now)
}

// isFullAt reports whether the bucket holds its whole burst at now. A full bucket
// behaves exactly like a newly created one, so it carries no state worth keeping.
func (tb *TokenBucket) isFullAt(now time.Time) bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	return tb.tokensAt(now) >= float64(tb.burst)
}

// take refills the bucket up to now and takes one token from it when there is one.
// The caller holds the mutex.
func (tb *TokenBucket) take(now time.Time) (bool, int) {
	tb.tokens = min(tb.tokensAt(now), float64(tb.burst))
	tb.lastUpdate = now
	if tb.tokens >= 1 {
		tb.tokens--
		return true, int(tb.tokens)
	}
	return false, int(tb.tokens)
}

// tokensAt returns the tokens the bucket would hold at now if the burst did not cap them.
// The caller holds the mutex.
func (tb *TokenBucket) tokensAt(now time.Time) float64 {
	return tb.tokens + now.Sub(tb.lastUpdate).Seconds()*float64(tb.rate)
}
