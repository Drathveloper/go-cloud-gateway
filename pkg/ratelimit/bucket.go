package ratelimit

import (
	"sync"
	"time"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
)

type TokenBucket struct {
	time       common.TimeProvider
	lastUpdate *time.Time
	tokens     float64
	rate       int
	burst      int
	mutex      sync.Mutex
}

func NewTokenBucket(time common.TimeProvider, rate, burst int) *TokenBucket {
	now := time.Now()
	return &TokenBucket{
		time:       time,
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastUpdate: &now,
	}
}

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
