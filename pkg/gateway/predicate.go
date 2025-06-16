package gateway

import (
	"net/http"
)

// Predicate represents a gateway predicate.
type Predicate interface {
	// Test returns true if the request should be forwarded to the backend.
	Test(request *http.Request) bool
	Name() string
}

// PredicateBuilder represents a predicate builder.
type PredicateBuilder interface {
	// The Build method is called to build a predicate with the given arguments. The arguments are passed from the
	// predicate configuration. The Build method should return an error if the predicate cannot be built with the given
	// arguments.
	Build(args map[string]any) (Predicate, error)
}

// PredicateBuilderFunc is a function that can be used as a predicate builder.
type PredicateBuilderFunc func(args map[string]any) (Predicate, error)

// Build calls f(args).
func (f PredicateBuilderFunc) Build(args map[string]any) (Predicate, error) {
	return f(args)
}

// PredicateBuilderRegistry is a registry of predicate builders.
//
// The PredicateBuilderRegistry type is a map that maps predicate names to predicate builders.
type PredicateBuilderRegistry map[string]PredicateBuilder

// Register registers the predicate builder with the given name.
func (r PredicateBuilderRegistry) Register(name string, builder PredicateBuilder) {
	r[name] = builder
}

// Predicates represent a list of predicates.
type Predicates []Predicate

// TestAll returns true if all predicates return true for the given request.
//
// If any predicate returns false, TestAll returns false.
//
// If all predicates return true, TestAll returns true.
//
// The order of the predicates in the list is important. The first predicate in the list is called first. The last
// predicate in the list is called last.
func (p Predicates) TestAll(req *http.Request) bool {
	for _, predicate := range p {
		if !predicate.Test(req) {
			return false
		}
	}
	return true
}
