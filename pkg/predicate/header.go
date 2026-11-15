package predicate

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"

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
//
// An empty regexp only requires the header to exist.
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
		// request headers are keyed by their canonical name: resolving it here keeps it out of every evaluation
		name:    http.CanonicalHeaderKey(header),
		pattern: pattern,
	}, nil
}

// NewHeaderPredicateBuilder creates a new header predicate builder.
//
// The 'name' argument is required. The 'regexp' argument is optional: without it the predicate
// only requires the header to exist.
func NewHeaderPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		name, err := shared.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		regex, err := optionalRegexpArg(args)
		if err != nil {
			return nil, err
		}
		return NewHeaderPredicate(name, regex)
	}
}

// Test checks if the header predicate matches the given request.
//
// If the header does not exist, the predicate will return false.
// If the header exists and there is no regexp, the predicate will return true, even when its value is empty.
// If the header exists and there is a regexp, the predicate will return true when at least one of
// its values (which can be empty) matches the regexp.
func (p *Header) Test(request *http.Request) bool {
	values := request.Header[p.name]
	if p.pattern == nil {
		return len(values) > 0
	}
	return slices.ContainsFunc(values, p.pattern.MatchString)
}

// Name returns the name of the predicate.
func (p *Header) Name() string {
	return HeaderPredicateName
}
