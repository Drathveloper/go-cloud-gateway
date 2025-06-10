package predicate

import "github.com/drathveloper/go-cloud-gateway/pkg/gateway"

// BuilderRegistry is the global predicate builder registry. It's initialized with the default predicate builders.
//
//nolint:gochecknoglobals
var BuilderRegistry gateway.PredicateBuilderRegistry = map[string]gateway.PredicateBuilder{
	MethodPredicateName:  NewMethodPredicateBuilder(),
	HostPredicateName:    NewHostPredicateBuilder(),
	PathPredicateName:    NewPathPredicateBuilder(),
	QueryPredicateName:   NewQueryPredicateBuilder(),
	HeaderPredicateName:  NewHeaderPredicateBuilder(),
	CookiePredicateName:  NewCookiePredicateBuilder(),
	BeforePredicateName:  NewBeforePredicateBuilder(),
	AfterPredicateName:   NewAfterPredicateBuilder(),
	BetweenPredicateName: NewBetweenPredicateBuilder(),
}
