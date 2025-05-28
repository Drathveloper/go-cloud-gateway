package gateway

import (
	"net/http"
)

type Predicate interface {
	Test(request *http.Request) bool
}

type PredicateBuilder interface {
	Build(args map[string]any) (Predicate, error)
}

type PredicateBuilderFunc func(args map[string]any) (Predicate, error)

func (f PredicateBuilderFunc) Build(args map[string]any) (Predicate, error) {
	return f(args)
}

type PredicateBuilderRegistry map[string]PredicateBuilder

func (r PredicateBuilderRegistry) Register(name string, builder PredicateBuilder) {
	r[name] = builder
}

type Predicates []Predicate

func (p Predicates) TestAll(req *http.Request) bool {
	for _, predicate := range p {
		if !predicate.Test(req) {
			return false
		}
	}
	return true
}
