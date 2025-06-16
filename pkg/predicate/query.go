package predicate

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// QueryPredicateName is the name of the query predicate.
const QueryPredicateName = "Query"

// Query is a predicate that checks if a query parameter exists and matches a given regexp.
type Query struct {
	pattern *regexp.Regexp
	name    string
}

// NewQueryPredicate creates a new query predicate.
//
// If the regexp is invalid, the predicate will return an error.
func NewQueryPredicate(name, regexpStr string) (*Query, error) {
	var pattern *regexp.Regexp
	var err error
	if regexpStr != "" {
		pattern, err = regexp.Compile(regexpStr)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp: %w", err)
		}
	}
	return &Query{
		name:    name,
		pattern: pattern,
	}, nil
}

// NewQueryPredicateBuilder creates a new query predicate builder.
func NewQueryPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		regex, err := common.ConvertToString(args["regexp"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'regexp' attribute: %w", err)
		}
		return NewQueryPredicate(name, regex)
	}
}

// Test checks if the query predicate matches the given request.
//
// If the query parameter does not exist, the predicate will return false.
// If the query parameter exists but does not match the regexp, the predicate will return false.
// If the query parameter exists and matches the regexp, the predicate will return true.
func (p *Query) Test(request *http.Request) bool {
	value := request.URL.Query().Get(p.name)
	if value == "" {
		return false
	}
	if p.pattern != nil {
		return p.pattern.MatchString(value)
	}
	return true
}

// Name returns the name of the predicate.
func (p *Query) Name() string {
	return QueryPredicateName
}
