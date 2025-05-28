package predicate

import (
	"fmt"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

const HostPredicateName = "Host"

type HostPredicate struct {
	Patterns []string
}

func NewHostPredicate(patterns ...string) *HostPredicate {
	return &HostPredicate{Patterns: patterns}
}

func NewHostPredicateBuilder() gateway.PredicateBuilder {
	return gateway.PredicateBuilderFunc(func(args map[string]any) (gateway.Predicate, error) {
		patterns, err := common.ConvertToStringSlice(args["patterns"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'patterns' attribute: %w", err)
		}
		return NewHostPredicate(patterns...), nil
	})
}

func (p *HostPredicate) Test(request *http.Request) bool {
	host := request.Host
	for _, pattern := range p.Patterns {
		if common.HostMatcher(pattern, host) {
			return true
		}
	}
	return false
}
