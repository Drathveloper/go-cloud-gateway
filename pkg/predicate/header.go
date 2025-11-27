package predicate

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// HeaderPredicateName is the name of the header predicate.
const HeaderPredicateName = "Header"

// Header is a predicate that checks if a header exists and matches a given regexp.
type Header struct {
	pattern *regexp.Regexp
	name    string
}

// NewHeaderPredicate creates a new header predicate.
func NewHeaderPredicate(header, regexpStr string) (*Header, error) {
	var pattern *regexp.Regexp
	var err error
	if regexpStr != "" {
		pattern, err = regexp.Compile(regexpStr)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp: %w", err)
		}
	}
	return &Header{
		name:    header,
		pattern: pattern,
	}, nil
}

// NewHeaderPredicateBuilder creates a new header predicate builder.
func NewHeaderPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		name, err := shared.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		regex, err := shared.ConvertToString(args["regexp"])
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
func (p *Header) Test(request *http.Request) bool {
	value := request.Header.Get(p.name)
	if value == "" {
		return false
	}
	if p.pattern != nil {
		return p.pattern.MatchString(value)
	}
	return true
}

// Name returns the name of the predicate.
func (p *Header) Name() string {
	return HeaderPredicateName
}
