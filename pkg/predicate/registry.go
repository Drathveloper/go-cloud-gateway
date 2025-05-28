package predicate

import "gateway/pkg/gateway"

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
