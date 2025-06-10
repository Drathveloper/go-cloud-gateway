package predicate

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// HeaderPredicateName is the name of the header predicate.
const HeaderPredicateName = "Header"

// HeaderPredicate is a predicate that checks if a header exists and matches a given regexp.
type HeaderPredicate struct {
	Pattern *regexp.Regexp
	Name    string
}

// NewHeaderPredicate creates a new header predicate.
func NewHeaderPredicate(header, regexpStr string) (*HeaderPredicate, error) {
	var pattern *regexp.Regexp
	var err error
	if regexpStr != "" {
		pattern, err = regexp.Compile(regexpStr)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp: %w", err)
		}
	}
	return &HeaderPredicate{
		Name:    header,
		Pattern: pattern,
	}, nil
}

// NewHeaderPredicateBuilder creates a new header predicate builder.
func NewHeaderPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		regex, err := common.ConvertToString(args["regexp"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'regexp' attribute: %w", err)
		}
		return NewHeaderPredicate(name, regex)
	}
}

// Test checks if the header predicate matches the given request.
//
// If the header does not exist, the predicate will return false.
// If the header exists but does not match the regexp, the predicate will return false.
// If the header exists and matches the regexp, the predicate will return true.
func (p *HeaderPredicate) Test(request *http.Request) bool {
	value := request.Header.Get(p.Name)
	if value == "" {
		return false
	}
	if p.Pattern != nil {
		return p.Pattern.MatchString(value)
	}
	return true
}
