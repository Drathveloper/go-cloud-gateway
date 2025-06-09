package filter

import (
	"errors"
	"fmt"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

var ErrFilterBuilder = errors.New("filter builder failed")

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
			return nil, fmt.Errorf("%w: filter %s and args %v", ErrFilterBuilder, name, args)
		}
		return fi, nil
	}
	return nil, fmt.Errorf("%w: filter builder not found for filter %s", ErrFilterBuilder, name)
}
