package predicate

import (
	"fmt"
	"gateway/pkg/common"
	"gateway/pkg/gateway"
	"net/http"
	"regexp"
)

const QueryPredicateName = "Query"

type QueryPredicate struct {
	Name    string
	Pattern *regexp.Regexp
}

func NewQueryPredicate(name, regexpStr string) (*QueryPredicate, error) {
	var pattern *regexp.Regexp
	var err error
	if regexpStr != "" {
		pattern, err = regexp.Compile(regexpStr)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp: %v", err)
		}
	}
	return &QueryPredicate{
		Name:    name,
		Pattern: pattern,
	}, nil
}

func NewQueryPredicateBuilder() gateway.PredicateBuilder {
	return gateway.PredicateBuilderFunc(func(args map[string]any) (gateway.Predicate, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		regex, err := common.ConvertToString(args["regexp"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'regexp' attribute: %w", err)
		}
		return NewQueryPredicate(name, regex)
	})
}

func (p *QueryPredicate) Test(request *http.Request) bool {
	values := request.URL.Query()[p.Name]
	if len(values) == 0 {
		return false
	}
	if p.Pattern != nil {
		for _, value := range values {
			if p.Pattern.MatchString(value) {
				return true
			}
		}
		return false
	}
	return true
}
