package ratelimit

import "time"

// RateLimiter is a rate limiter.
type RateLimiter interface {
	// Allow check if the given key is allowed to pass. Returns true if allowed, false if not allowed, and the remaining
	// tokens.
	Allow(key string) (bool, int)
}

// RetryAfterRateLimiter is an optional interface a RateLimiter may also implement to
// report how long a rejected key has to wait before it is allowed again.
//
// It is separate from RateLimiter so limiters written against that interface keep
// working unchanged: a limiter without it simply reports no wait. When a limiter has it,
// the RateLimit filter calls AllowWithRetryAfter instead of Allow, never both.
type RetryAfterRateLimiter interface {
	RateLimiter
	// AllowWithRetryAfter is Allow that also returns, for a rejected key, the time until
	// the key is allowed again. The decision and the wait come from one call so they
	// describe the same state of the limiter: asking for the wait after Allow would race
	// with the requests made in between.
	//
	// The wait is only meaningful for a rejected key. A wait of zero or less means the
	// limiter cannot tell.
	AllowWithRetryAfter(key string) (bool, int, time.Duration)
}

// RateLimiterBuilder is a RateLimiter builder.
type RateLimiterBuilder interface {
	// The Build method is called to build a rate limiter with the given arguments. The arguments are passed from the
	// rate limiter configuration. The Build method should return an error if the rate limiter cannot be built with the
	// given arguments.
	Build(args map[string]any) (RateLimiter, error)
}

// RateLimiterBuilderFunc is a rate limiter builder.
//
// The Build method is called to build a rate limiter with the given arguments. The arguments are passed from the rate
// limiter configuration. The Build method should return an error if the rate limiter cannot be built with the given
// arguments.
//
// The args are expected to be a map of strings to any.
type RateLimiterBuilderFunc func(args map[string]any) (RateLimiter, error)

// Build calls rate limiter builder func.
//
//nolint:ireturn
func (f RateLimiterBuilderFunc) Build(args map[string]any) (RateLimiter, error) {
	return f(args)
}
