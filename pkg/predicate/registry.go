package predicate

import "github.com/drathveloper/go-cloud-gateway/pkg/gateway"

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
