package predicate

import (
	"fmt"
	"gateway/pkg/gateway"
)

type Factory struct {
	registry map[string]gateway.PredicateBuilder
}

func NewFactory(registry gateway.PredicateBuilderRegistry) *Factory {
	return &Factory{
		registry: registry,
	}
}

func (f *Factory) Build(name string, args map[string]any) (gateway.Predicate, error) {
	if f.registry[name] != nil {
		fi, err := f.registry[name].Build(args)
		if err != nil {
			return nil, fmt.Errorf("predicate builder failed for predicate %s and args %v", name, args)
		}
		return fi, nil
	}
	return nil, fmt.Errorf("predicate builder not found for predicate %s", name)
}
