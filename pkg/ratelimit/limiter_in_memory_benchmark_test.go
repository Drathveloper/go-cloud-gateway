package ratelimit_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/ratelimit"
)

func BenchmarkInMemoryRateLimiter_Allow_ExistingKey(b *testing.B) {
	limiter := ratelimit.NewInMemoryRateLimiter(&shared.RealTime{}, 1000, 1000)
	limiter.Allow("key")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		limiter.Allow("key")
	}
}

// Every call uses a key never seen before, the pattern of a client forging its
// X-Forwarded-For header. The cost includes building the key, creating its bucket and,
// amortized, the sweeps that delete the buckets left behind. The fake clock advances one
// millisecond per call so the run covers enough limiter time for the sweeps to happen.
func BenchmarkInMemoryRateLimiter_Allow_DistinctKeys(b *testing.B) {
	clock := &MockTimeProvider{WantedTime: time.Now(), Increment: 1}
	limiter := ratelimit.NewInMemoryRateLimiter(clock, 10, 10)

	b.ReportAllocs()
	b.ResetTimer()
	var i int
	for b.Loop() {
		limiter.Allow(strconv.Itoa(i))
		i++
	}
}
