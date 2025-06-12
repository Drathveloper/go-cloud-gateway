package filter

import "github.com/drathveloper/go-cloud-gateway/pkg/gateway"

// BuilderRegistry is a filter builder registry.
//
// The key is the filter name.
// The value is the filter builder.
//
//nolint:gochecknoglobals
var BuilderRegistry gateway.FilterBuilderRegistry = map[string]gateway.FilterBuilder{
	AddRequestHeaderFilterName:      NewAddRequestHeaderBuilder(),
	SetRequestHeaderFilterName:      NewSetRequestHeaderBuilder(),
	RemoveRequestHeaderFilterName:   NewRemoveRequestHeaderBuilder(),
	AddResponseHeaderFilterName:     NewAddResponseHeaderBuilder(),
	SetResponseHeaderFilterName:     NewSetResponseHeaderBuilder(),
	RemoveResponseHeaderFilterName:  NewRemoveResponseHeaderBuilder(),
	RequestResponseLoggerFilterName: NewRequestResponseLoggerBuilder(),
	RewritePathFilterName:           NewRewritePathBuilder(),
	RateLimitFilterName:             NewRateLimitBuilder(),
}
