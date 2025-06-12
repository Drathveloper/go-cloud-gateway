package ratelimit

// RateLimiter is a rate limiter.
type RateLimiter interface {
	// Allow check if the given key is allowed to pass. Returns true if allowed, false if not allowed, and the remaining
	// tokens.
	Allow(key string) (bool, int)
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
func (f RateLimiterBuilderFunc) Build(args map[string]any) (RateLimiter, error) {
	return f(args)
}
