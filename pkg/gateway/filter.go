package gateway

import (
	"fmt"
)

type Filter interface {
	PreProcess(ctx *Context) error
	PostProcess(ctx *Context) error
	Name() string
}

type FilterBuilder interface {
	Build(args map[string]any) (Filter, error)
}

type FilterBuilderFunc func(args map[string]any) (Filter, error)

func (f FilterBuilderFunc) Build(args map[string]any) (Filter, error) {
	return f(args)
}

type FilterBuilderRegistry map[string]FilterBuilder

func (r FilterBuilderRegistry) Register(name string, builder FilterBuilder) {
	r[name] = builder
}

type Filters []Filter

func (f Filters) PreProcessAll(ctx *Context) error {
	for _, filter := range f {
		if err := filter.PreProcess(ctx); err != nil {
			return fmt.Errorf("pre-process filters failed with filter %s: %w", filter.Name(), err)
		}
	}
	return nil
}

func (f Filters) PostProcessAll(ctx *Context) error {
	for i := len(f) - 1; i >= 0; i-- {
		if err := f[i].PostProcess(ctx); err != nil {
			return fmt.Errorf("post-process filters failed with filter %s: %w", f[i].Name(), err)
		}
	}
	return nil
}
