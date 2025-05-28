package filter

import (
	"fmt"
	"gateway/pkg/gateway"
)

type Factory struct {
	registry map[string]gateway.FilterBuilder
}

func NewFactory(registry gateway.FilterBuilderRegistry) *Factory {
	return &Factory{
		registry: registry,
	}
}

func (f *Factory) Build(name string, args map[string]any) (gateway.Filter, error) {
	if f.registry[name] != nil {
		fi, err := f.registry[name].Build(args)
		if err != nil {
			return nil, fmt.Errorf("filter builder failed for filter %s and args %v", name, args)
		}
		return fi, nil
	}
	return nil, fmt.Errorf("filter builder not found for filter %s", name)
}
