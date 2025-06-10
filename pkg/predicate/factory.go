package predicate

import (
	"errors"
	"fmt"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// ErrInvalidPredicate is returned when the predicate args are invalid.
var ErrInvalidPredicate = errors.New("invalid predicate args")

// Factory is a predicate factory.
type Factory struct {
	registry map[string]gateway.PredicateBuilder
}

// NewFactory creates a new predicate factory.
func NewFactory(registry gateway.PredicateBuilderRegistry) *Factory {
	return &Factory{
		registry: registry,
	}
}

// Build builds a predicate from the given name and args.
//
// If the predicate builder is not found, the factory will return an error.
// If the predicate builder is found but the args are invalid, the factory will return an error.
// If the predicate builder is found and the args are valid, the factory will return a predicate.
//
// The args are expected to be a map of strings to any.
func (f *Factory) Build(name string, args map[string]any) (gateway.Predicate, error) {
	if f.registry[name] != nil {
		fi, err := f.registry[name].Build(args)
		if err != nil {
			return nil, fmt.Errorf("%w: name %s and args %v", ErrInvalidPredicate, name, args)
		}
		return fi, nil
	}
	return nil, fmt.Errorf("%w: name: %s", ErrInvalidPredicate, name)
}
