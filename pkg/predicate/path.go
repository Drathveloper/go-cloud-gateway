package predicate

import (
	"fmt"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

const PathPredicateName = "Path"

type Path struct {
	patterns []string
}

func NewPathPredicate(patterns ...string) *Path {
	return &Path{
		patterns: patterns,
	}
}

func NewPathPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		patterns, err := common.ConvertToStringSlice(args["patterns"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'patterns' attribute: %w", err)
		}
		return NewPathPredicate(patterns...), nil
	}
}

func (p *Path) Test(r *http.Request) bool {
	for _, pattern := range p.patterns {
		if common.PathMatcher(pattern, r.URL.Path) {
			return true
		}
	}
	return false
}

func (p *Path) Name() string {
	return PathPredicateName
}
