package predicate

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// CookiePredicateName is the name of the cookie predicate.
const CookiePredicateName = "Cookie"

// ErrInvalidCookieRegexp is returned when the cookie predicate's regexp is invalid.
var ErrInvalidCookieRegexp = errors.New("invalid cookie regexp")

// CookiePredicate is a predicate that checks if a cookie exists and matches a given regexp.
type CookiePredicate struct {
	Pattern *regexp.Regexp
	Name    string
}

// NewCookiePredicate creates a new cookie predicate.
func NewCookiePredicate(name, regexpStr string) (*CookiePredicate, error) {
	var pattern *regexp.Regexp
	var err error
	if regexpStr != "" {
		pattern, err = regexp.Compile(regexpStr)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidCookieRegexp, err.Error())
		}
	}
	return &CookiePredicate{
		Name:    name,
		Pattern: pattern,
	}, nil
}

// NewCookiePredicateBuilder creates a new cookie predicate builder.
func NewCookiePredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		regex, err := common.ConvertToString(args["regexp"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'regexp' attribute: %w", err)
		}
		return NewCookiePredicate(name, regex)
	}
}

// Test checks if the cookie predicate matches the given request.
//
// If the cookie does not exist, the predicate will return false.
// If the cookie exists but does not match the regexp, the predicate will return false.
// If the cookie exists and matches the regexp, the predicate will return true.
func (p *CookiePredicate) Test(request *http.Request) bool {
	cookies := request.Cookies()
	for _, cookie := range cookies {
		if cookie.Name == p.Name {
			if p.Pattern != nil {
				return p.Pattern.MatchString(cookie.Value)
			}
			return true
		}
	}
	return false
}
