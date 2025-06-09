package filter

import "github.com/drathveloper/go-cloud-gateway/pkg/gateway"

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
}
