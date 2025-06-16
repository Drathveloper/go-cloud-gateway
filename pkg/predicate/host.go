package predicate

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// HostPredicateName is the name of the host predicate.
const HostPredicateName = "Host"

// Host is a predicate that checks if a host matches a given pattern.
type Host struct {
	patterns      []string
	compiledRegex []*regexp.Regexp
}

// NewHostPredicate creates a new host predicate.
func NewHostPredicate(patterns ...string) (*Host, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern == "**" {
			compiled = append(compiled, nil)
			continue
		}
		r := common.ConvertPatternToRegex(pattern)
		re, err := regexp.Compile(r)
		if err != nil {
			return nil, fmt.Errorf("invalid host pattern %q: %w", pattern, err)
		}
		compiled = append(compiled, re)
	}
	return &Host{
		patterns:      patterns,
		compiledRegex: compiled,
	}, nil
}

// NewHostPredicateBuilder creates a new host predicate builder.
func NewHostPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		patterns, err := common.ConvertToStringSlice(args["patterns"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'patterns' attribute: %w", err)
		}
		return NewHostPredicate(patterns...)
	}
}

// Test checks if the host matches the given request.
//
// If the host does not match any pattern, the predicate will return false.
// If the host matches at least one pattern, the predicate will return true. .
func (p *Host) Test(request *http.Request) bool {
	for _, pattern := range p.compiledRegex {
		if common.HostMatcher(pattern, request.Host) {
			return true
		}
	}
	return false
}

// Name returns the name of the predicate.
func (p *Host) Name() string {
	return HostPredicateName
}
