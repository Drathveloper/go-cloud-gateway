package predicate

import (
	"fmt"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// PathPredicateName is the name of the path predicate.
const PathPredicateName = "Path"

// Path is a predicate that checks if the request path matches a given pattern.
type Path struct {
	patterns []string
}

// NewPathPredicate creates a new path predicate.
func NewPathPredicate(patterns ...string) *Path {
	return &Path{
		patterns: patterns,
	}
}

// NewPathPredicateBuilder creates a new path predicate builder.
func NewPathPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		patterns, err := common.ConvertToStringSlice(args["patterns"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'patterns' attribute: %w", err)
		}
		return NewPathPredicate(patterns...), nil
	}
}

// Test checks if the request path matches the given patterns.
//
// If the request path does not match any pattern, the predicate will return false.
// If the request path matches at least one pattern, the predicate will return true.
func (p *Path) Test(r *http.Request) bool {
	for _, pattern := range p.patterns {
		if common.PathMatcher(pattern, r.URL.Path) {
			return true
		}
	}
	return false
}
