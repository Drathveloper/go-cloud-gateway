package predicate

import (
	"errors"
	"fmt"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

var ErrInvalidPredicate = errors.New("invalid predicate args")

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
			return nil, fmt.Errorf("%w: name %s and args %v", ErrInvalidPredicate, name, args)
		}
		return fi, nil
	}
	return nil, fmt.Errorf("%w: name: %s", ErrInvalidPredicate, name)
}
