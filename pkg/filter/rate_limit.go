package filter

import (
	"errors"
	"fmt"
	"time"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/ratelimit"
)

// ErrRateLimitExceeded is returned when the rate limit is exceeded. The RateLimit filter
// returns it as a RateLimitExceededError, which errors.Is matches against it.
var ErrRateLimitExceeded = errors.New("rate limit exceeded")

// ErrInvalidRateLimitKey is returned when the rate limit key is invalid.
var ErrInvalidRateLimitKey = errors.New("invalid rate limit key")

// ErrInvalidRateLimitType is returned when the rate limit type is invalid.
var ErrInvalidRateLimitType = errors.New("invalid rate limit type")

// RateLimitFilterName is the name of the rate limit filter.
const RateLimitFilterName = "RateLimit"

// RateLimitExceededError is the error the RateLimit filter returns for a rejected request.
//
// It unwraps to ErrRateLimitExceeded, so errors.Is(err, ErrRateLimitExceeded) holds for
// it. An error handler that needs the details gets them with
// errors.AsType[*RateLimitExceededError](err).
type RateLimitExceededError struct {
	// RetryAfter is how long the client has to wait before a request can be allowed
	// again. It is zero when the limiter cannot tell, which is the case of every limiter
	// that does not implement ratelimit.RetryAfterRateLimiter.
	RetryAfter time.Duration
	// Remaining is the number of tokens the limiter reported as left.
	Remaining int
}

// Error returns the error message. The wait is only part of it when it is known.
func (e *RateLimitExceededError) Error() string {
	if e.RetryAfter <= 0 {
		return fmt.Sprintf("%s: remaining %d", ErrRateLimitExceeded.Error(), e.Remaining)
	}
	return fmt.Sprintf("%s: remaining %d, retry after %s", ErrRateLimitExceeded.Error(), e.Remaining, e.RetryAfter)
}

// Unwrap returns ErrRateLimitExceeded.
func (e *RateLimitExceededError) Unwrap() error {
	return ErrRateLimitExceeded
}

// RateLimit is a filter that limits the number of requests per second.
//
// The rate limiter is used to check if the request is allowed to proceed.
// The key func is used to get the key for the rate limiter.
type RateLimit struct {
	limiter ratelimit.RateLimiter
	// retryAfterLimiter is the limiter when it can also report how long a rejected key
	// has to wait, nil otherwise. It is resolved once here rather than on every request.
	retryAfterLimiter ratelimit.RetryAfterRateLimiter
	keyFunc           ratelimit.KeyFunc
}

// NewRateLimitFilter creates a new RateLimitFilter.
//
// A limiter that also implements ratelimit.RetryAfterRateLimiter makes the filter report
// how long a rejected request has to wait.
func NewRateLimitFilter(limiter ratelimit.RateLimiter, keyFunc ratelimit.KeyFunc) *RateLimit {
	retryAfterLimiter, _ := limiter.(ratelimit.RetryAfterRateLimiter)
	return &RateLimit{
		limiter:           limiter,
		retryAfterLimiter: retryAfterLimiter,
		keyFunc:           keyFunc,
	}
}

// NewRateLimitBuilder creates a new RateLimitBuilder.
//
// The args are expected to be a map of strings to any.
//
// The args are expected to contain the following keys:
// - type: the type of the rate limiter.
// - key: the key of the rate limiter.
// Other specific args are expected to be passed to the rate limiter and key func builders depending on the
// implementation details.
func NewRateLimitBuilder() gateway.FilterBuilderFunc {
	return func(args map[string]any) (gateway.Filter, error) {
		rateLimitType, err := shared.ConvertToString(args["type"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'type' attribute: %w", err)
		}
		rateLimitKey, err := shared.ConvertToString(args["key"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'key' attribute: %w", err)
		}
		keyFuncBuilder, isPresent := ratelimit.KeyFuncBuilderRegistry[rateLimitKey]
		if !isPresent {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRateLimitKey, rateLimitKey)
		}
		keyFunc, err := keyFuncBuilder.Build(args)
		if err != nil {
			return nil, fmt.Errorf("failed to build rate limit key: %w", err)
		}
		rateLimitBuilder, isPresent := ratelimit.RateLimiterBuilderRegistry[rateLimitType]
		if !isPresent {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRateLimitType, rateLimitType)
		}
		rateLimiter, err := rateLimitBuilder.Build(args)
		if err != nil {
			return nil, fmt.Errorf("failed to build rate limiter: %w", err)
		}
		return NewRateLimitFilter(rateLimiter, keyFunc), nil
	}
}

// PreProcess checks if the request is allowed to proceed.
// If the request is not allowed to proceed, the filter will return a RateLimitExceededError, which wraps
// ErrRateLimitExceeded and carries the remaining requests and, when the limiter reports it, the time to wait.
// If the request is allowed to proceed, the filter will return nil.
func (f *RateLimit) PreProcess(ctx *gateway.Context) error {
	allowed, remaining, retryAfter := f.allow(f.keyFunc(ctx))
	if allowed {
		return nil
	}
	return &RateLimitExceededError{
		Remaining: remaining,
		// A limiter that cannot tell may answer with anything up to zero.
		RetryAfter: max(retryAfter, 0),
	}
}

// PostProcess does nothing.
func (f *RateLimit) PostProcess(_ *gateway.Context) error {
	return nil
}

// Name returns the name of the filter.
func (f *RateLimit) Name() string {
	return RateLimitFilterName
}

// allow asks the limiter once. The decision and the wait come from that single call:
// reading the wait after Allow would describe the limiter after other requests changed it.
func (f *RateLimit) allow(key string) (bool, int, time.Duration) {
	if f.retryAfterLimiter != nil {
		return f.retryAfterLimiter.AllowWithRetryAfter(key)
	}
	allowed, remaining := f.limiter.Allow(key)
	return allowed, remaining, 0
}
