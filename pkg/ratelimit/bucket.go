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

// allowOrWaitAt is allowAt that also returns, for a rejected request, the time until the
// bucket holds one token again, or zero when it never will. The wait is computed under
// the same lock as the decision, so no other request can change the bucket between the
// two. It is a separate method because the wait costs about as much as the decision, and
// a caller that only wants the decision should not pay for it.
func (tb *TokenBucket) allowOrWaitAt(now time.Time) (bool, int, time.Duration) {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	allowed, remaining := tb.take(now)
	if allowed {
		return true, remaining, 0
	}
	return false, remaining, tb.waitForToken()
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

// waitForToken returns the time until the bucket holds one token again, counted from its
// last update, or zero when it never will: a bucket that does not refill, or whose burst
// is too small to hold a token. The caller holds the mutex and has just called take, so
// the last update is the instant of the rejected request.
func (tb *TokenBucket) waitForToken() time.Duration {
	if tb.rate <= 0 || tb.burst < 1 {
		return 0
	}
	wait := time.Duration((1 - tb.tokens) / float64(tb.rate) * float64(time.Second))
	// The conversion truncates, and the division can land a hair under an exact result. A
	// caller coming back after such a wait would find the bucket a fraction short of a
	// token and be rejected again, so the wait is moved up to the first nanosecond at
	// which take finds a whole token. A nanosecond adds far more tokens than the rounding
	// loses, so this takes a step or two.
	for tb.tokensAfter(wait) < 1 {
		wait++
	}
	return wait
}

// tokensAt returns the tokens the bucket would hold at now if the burst did not cap them.
// The caller holds the mutex.
func (tb *TokenBucket) tokensAt(now time.Time) float64 {
	return tb.tokensAfter(now.Sub(tb.lastUpdate))
}

// tokensAfter returns the tokens the bucket would hold once elapsed has passed since its
// last update if the burst did not cap them. The refill and the wait both go through it,
// so they cannot disagree on the instant a token becomes available.
// The caller holds the mutex.
func (tb *TokenBucket) tokensAfter(elapsed time.Duration) float64 {
	return tb.tokens + elapsed.Seconds()*float64(tb.rate)
}
