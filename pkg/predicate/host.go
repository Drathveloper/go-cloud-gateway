package predicate

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

const HostPredicateName = "Host"

type HostPredicate struct {
	Patterns      []string
	compiledRegex []*regexp.Regexp
}

func NewHostPredicate(patterns ...string) (*HostPredicate, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if p == "**" {
			compiled = append(compiled, nil)
			continue
		}
		r := common.ConvertPatternToRegex(p)
		re, err := regexp.Compile(r)
		if err != nil {
			return nil, fmt.Errorf("invalid host pattern %q: %w", p, err)
		}
		compiled = append(compiled, re)
	}
	return &HostPredicate{
		Patterns:      patterns,
		compiledRegex: compiled,
	}, nil
}

func NewHostPredicateBuilder() gateway.PredicateBuilder {
	return gateway.PredicateBuilderFunc(func(args map[string]any) (gateway.Predicate, error) {
		patterns, err := common.ConvertToStringSlice(args["patterns"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'patterns' attribute: %w", err)
		}
		return NewHostPredicate(patterns...)
	})
}

func (p *HostPredicate) Test(request *http.Request) bool {
	for _, pattern := range p.compiledRegex {
		if common.HostMatcher(pattern, request.Host) {
			return true
		}
	}
	return false
}
