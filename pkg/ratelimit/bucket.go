package ratelimit

import (
	"sync"
	"time"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

// TokenBucket is a token bucket.
type TokenBucket struct {
	time       shared.TimeProvider
	lastUpdate *time.Time
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
	now := time.Now()
	return &TokenBucket{
		time:       time,
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastUpdate: &now,
	}
}

// Allow checks if the token bucket allows the request.
//
// If the token bucket allows the request, Allow returns true and the remaining tokens.
// If the token bucket does not allow the request, Allow returns false and the remaining tokens.
func (tb *TokenBucket) Allow() (bool, int) {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	now := tb.time.Now()
	elapsed := now.Sub(*tb.lastUpdate).Seconds()
	tb.tokens += elapsed * float64(tb.rate)
	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}
	tb.lastUpdate = &now
	if tb.tokens >= 1 {
		tb.tokens--
		return true, int(tb.tokens)
	}
	return false, int(tb.tokens)
}
