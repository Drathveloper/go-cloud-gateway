package gateway

import (
	"fmt"
)

// Filter represents a gateway filter.
type Filter interface {
	// The PreProcess method is called before the request is forwarded to the backend. PreProcess should only modify the
	// request and return error if the request should not be forwarded to the backend.
	PreProcess(ctx *Context) error
	// The PostProcess method is called after the response is received from the backend. PostProcess should only modify
	// the response and return error if the original response should not be returned to the client.
	PostProcess(ctx *Context) error
	// The Name method returns the name of the filter.
	Name() string
}

// FilterBuilder represents a filter builder.
type FilterBuilder interface {
	// The Build method is called to build a filter with the given arguments. The arguments are passed from the filter
	// configuration. The Build method should return an error if the filter cannot be built with the given arguments.
	Build(args map[string]any) (Filter, error)
}

// FilterBuilderFunc is a function that can be used as a filter builder.
//
// The Build method is called to build a filter with the given arguments. The arguments are passed from the filter
// configuration. The Build method should return an error if the filter cannot be built with the given arguments.
//
// The FilterBuilderFunc type is an adapter to allow the use of ordinary functions as filter builders. If f is a
// function with the appropriate signature, FilterBuilderFunc(f) is a FilterBuilder that calls f.
type FilterBuilderFunc func(args map[string]any) (Filter, error)

// Build calls f(args).
//
//nolint:ireturn
func (f FilterBuilderFunc) Build(args map[string]any) (Filter, error) {
	return f(args)
}

// FilterBuilderRegistry is a registry of filter builders.
//
// The FilterBuilderRegistry type is a map that maps filter names to filter builders.
type FilterBuilderRegistry map[string]FilterBuilder

// Register registers the filter builder with the given name.
func (r FilterBuilderRegistry) Register(name string, builder FilterBuilder) {
	r[name] = builder
}

// Filters represent a list of filters.
type Filters []Filter

// PreProcessAll calls PreProcess on each filter in the list.
//
// If any filter returns an error, PreProcessAll returns the error.
//
// If all filters return nil, PreProcessAll returns nil.
//
// The order of the filters in the list is important. The first filter in the list is called first. The last filter in
// the list is called last.
func (f Filters) PreProcessAll(ctx *Context) error {
	for _, filter := range f {
		if err := filter.PreProcess(ctx); err != nil {
			name := filter.Name()
			return fmt.Errorf("pre-process filters failed with filter %s: %w", name, err)
		}
	}
	return nil
}

// PostProcessAll calls PostProcess on each filter in the list in reverse order.
//
// If any filter returns an error, PostProcessAll returns the error.
//
// If all filters return nil, PostProcessAll returns nil.
//
// The order of the filters in the list is important. The first filter in the list is called last. The last filter in
// the list is called first.
func (f Filters) PostProcessAll(ctx *Context) error {
	for i := len(f) - 1; i >= 0; i-- {
		if err := f[i].PostProcess(ctx); err != nil {
			name := f[i].Name()
			return fmt.Errorf("post-process filters failed with filter %s: %w", name, err)
		}
	}
	return nil
}
