package filter

import (
	"errors"
	"fmt"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/ratelimit"
)

// ErrRateLimitExceeded is returned when the rate limit is exceeded.
var ErrRateLimitExceeded = errors.New("rate limit exceeded")

// ErrInvalidRateLimitKey is returned when the rate limit key is invalid.
var ErrInvalidRateLimitKey = errors.New("invalid rate limit key")

// ErrInvalidRateLimitType is returned when the rate limit type is invalid.
var ErrInvalidRateLimitType = errors.New("invalid rate limit type")

// RateLimitFilterName is the name of the rate limit filter.
const RateLimitFilterName = "RateLimit"

// RateLimit is a filter that limits the number of requests per second.
//
// The rate limiter is used to check if the request is allowed to proceed.
// The key func is used to get the key for the rate limiter.
type RateLimit struct {
	limiter ratelimit.RateLimiter
	keyFunc ratelimit.KeyFunc
}

// NewRateLimitFilter creates a new RateLimitFilter.
func NewRateLimitFilter(limiter ratelimit.RateLimiter, keyFunc ratelimit.KeyFunc) *RateLimit {
	return &RateLimit{
		limiter: limiter,
		keyFunc: keyFunc,
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
		rateLimitType, err := common.ConvertToString(args["type"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'type' attribute: %w", err)
		}
		rateLimitKey, err := common.ConvertToString(args["key"])
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
// If the request is not allowed to proceed, the filter will return an ErrRateLimitExceeded error with the remaining
// requests as the error message.
// If the request is allowed to proceed, the filter will return nil.
func (f *RateLimit) PreProcess(ctx *gateway.Context) error {
	key := f.keyFunc(ctx)
	if allowed, remaining := f.limiter.Allow(key); !allowed {
		return fmt.Errorf("%w: remaining %d", ErrRateLimitExceeded, remaining)
	}
	return nil
}

// PostProcess does nothing.
func (f *RateLimit) PostProcess(_ *gateway.Context) error {
	return nil
}

// Name returns the name of the filter.
func (f *RateLimit) Name() string {
	return RateLimitFilterName
}
