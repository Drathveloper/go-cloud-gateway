package filter

import "gateway/pkg/gateway"

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
