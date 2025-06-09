package predicate

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
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
			return nil, fmt.Errorf("invalid regexp: %w", err)
		}
	}
	return &QueryPredicate{
		Name:    name,
		Pattern: pattern,
	}, nil
}

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

func (p *QueryPredicate) Test(request *http.Request) bool {
	value := request.URL.Query().Get(p.Name)
	if value == "" {
		return false
	}
	if p.Pattern != nil {
		if p.Pattern.MatchString(value) {
			return true
		}
		return false
	}
	return true
}
