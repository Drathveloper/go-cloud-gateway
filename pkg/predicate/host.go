package predicate

import (
	"fmt"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// HostPredicateName is the name of the host predicate.
const HostPredicateName = "Host"

// Host is a predicate that checks if a host matches a given pattern.
type Host struct {
	patterns []shared.HostPattern
}

// NewHostPredicate creates a new host predicate.
//
// A pattern can contain '*' (one or more characters within a label), '**' (one or more
// characters across labels) and '?' (exactly one character); everything else is literal.
// Matching is case-insensitive and ignores the port of the request host, unless the
// pattern has a port itself.
func NewHostPredicate(patterns ...string) (*Host, error) {
	compiled := make([]shared.HostPattern, 0, len(patterns))
	for _, pattern := range patterns {
		hostPattern, err := shared.CompileHostPattern(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid host pattern %q: %w", pattern, err)
		}
		compiled = append(compiled, hostPattern)
	}
	return &Host{
		patterns: compiled,
	}, nil
}

// NewHostPredicateBuilder creates a new host predicate builder.
func NewHostPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		patterns, err := shared.ConvertToStringSlice(args["patterns"])
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
	for i := range p.patterns {
		if p.patterns[i].Match(request.Host) {
			return true
		}
	}
	return false
}

// Name returns the name of the predicate.
func (p *Host) Name() string {
	return HostPredicateName
}
