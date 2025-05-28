package predicate

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

const HeaderPredicateName = "Header"

type HeaderPredicate struct {
	Name    string
	Pattern *regexp.Regexp
}

func NewHeaderPredicate(header, regexpStr string) (*HeaderPredicate, error) {
	var pattern *regexp.Regexp
	var err error
	if regexpStr != "" {
		pattern, err = regexp.Compile(regexpStr)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp: %v", err)
		}
	}
	return &HeaderPredicate{
		Name:    header,
		Pattern: pattern,
	}, nil
}

func NewHeaderPredicateBuilder() gateway.PredicateBuilder {
	return gateway.PredicateBuilderFunc(func(args map[string]any) (gateway.Predicate, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		regex, err := common.ConvertToString(args["regexp"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'regexp' attribute: %w", err)
		}
		return NewHeaderPredicate(name, regex)
	})
}

func (p *HeaderPredicate) Test(request *http.Request) bool {
	values := request.Header.Values(p.Name)
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
