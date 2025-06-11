package filter

import (
	"errors"
	"fmt"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/ratelimit"
)

var ErrRateLimitExceeded = errors.New("rate limit exceeded")

const RateLimitFilterName = "RateLimit"

type RateLimit struct {
	limiter ratelimit.RateLimiter
	keyFunc RateLimitKeyFunc
}

func NewRateLimitFilter(limiter ratelimit.RateLimiter, keyFunc RateLimitKeyFunc) *RateLimit {
	return &RateLimit{
		limiter: limiter,
		keyFunc: keyFunc,
	}
}

func (f *RateLimit) PreProcess(ctx *gateway.Context) error {
	key := f.keyFunc(ctx)
	if allowed, remaining := f.limiter.Allow(key); !allowed {
		return fmt.Errorf("%w: remaining %d", ErrRateLimitExceeded, remaining)
	}
	return nil
}

func (f *RateLimit) PostProcess(_ *gateway.Context) error {
	return nil
}

func (f *RateLimit) Name() string {
	return RateLimitFilterName
}

type RateLimitKeyFunc func(ctx *gateway.Context) string

/*func IPRateLimitKey(ctx *gateway.Context) string {
	ip, ok := ctx.Request.Headers["X-Forwarded-For"]
	if !ok {
		return "unknown"
	}
}*/
