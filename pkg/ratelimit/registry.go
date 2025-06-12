package ratelimit

// KeyFuncRegistry is a key func builder registry.
type KeyFuncRegistry map[string]KeyFuncBuilder

// Register registers the key func builder with the given name.
func (r KeyFuncRegistry) Register(name string, keyFuncBuilder KeyFuncBuilder) {
	r[name] = keyFuncBuilder
}

// KeyFuncBuilderRegistry is a key func builder registry.
//
// The KeyFuncBuilderRegistry type is a map that maps key func names to key func builders.
//
//nolint:gochecknoglobals
var KeyFuncBuilderRegistry KeyFuncRegistry = map[string]KeyFuncBuilder{
	IPKeyFunc:         NewIPKeyFuncBuilder(),
	PathKeyFunc:       NewPathKeyFuncBuilder(),
	PathMethodKeyFunc: NewPathAndMethodKeyFuncBuilder(),
	QueryKeyFunc:      NewQueryKeyFuncBuilder(),
	HeaderKeyFunc:     NewHeaderKeyFuncBuilder(),
}

// RateLimiterRegistry is a rate limiter builder registry.
type RateLimiterRegistry map[string]RateLimiterBuilder

// Register registers the rate limiter builder with the given name.
func (r RateLimiterRegistry) Register(name string, rateLimiterBuilder RateLimiterBuilder) {
	r[name] = rateLimiterBuilder
}

// RateLimiterBuilderRegistry is a rate limiter builder registry.
//
// The RateLimiterBuilderRegistry type is a map that maps rate limiter names to rate limiter builders.
//
//nolint:gochecknoglobals
var RateLimiterBuilderRegistry RateLimiterRegistry = map[string]RateLimiterBuilder{
	InMemoryRateLimiterName: NewInMemoryRateLimiterBuilder(),
}
