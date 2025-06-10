package filter

import (
	"errors"
	"fmt"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// ErrFilterBuilder is returned when the filter builder fails.
var ErrFilterBuilder = errors.New("filter builder failed")

// Factory is a filter factory.
type Factory struct {
	registry map[string]gateway.FilterBuilder
}

// NewFactory creates a new filter factory.
func NewFactory(registry gateway.FilterBuilderRegistry) *Factory {
	return &Factory{
		registry: registry,
	}
}

// Build builds a filter from the given name and args.
//
// If the filter builder is not found, the factory will return an error.
// If the filter builder is found but the args are invalid, the factory will return an error.
// If the filter builder is found and the args are valid, the factory will return a filter.
//
// The args are expected to be a map of strings to any.
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
