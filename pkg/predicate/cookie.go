package predicate

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// CookiePredicateName is the name of the cookie predicate.
const CookiePredicateName = "Cookie"

// ErrInvalidCookieRegexp is returned when the cookie predicate's regexp is invalid.
var ErrInvalidCookieRegexp = errors.New("invalid cookie regexp")

// Cookie is a predicate that checks if a cookie exists and matches a given regexp.
type Cookie struct {
	pattern *regexp.Regexp
	name    string
}

// NewCookiePredicate creates a new cookie predicate.
//
// An empty regexp only requires the cookie to exist.
func NewCookiePredicate(name, regexpStr string) (*Cookie, error) {
	var pattern *regexp.Regexp
	var err error
	if regexpStr != "" {
		pattern, err = regexp.Compile(regexpStr)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidCookieRegexp, err.Error())
		}
	}
	return &Cookie{
		name:    name,
		pattern: pattern,
	}, nil
}

// NewCookiePredicateBuilder creates a new cookie predicate builder.
//
// The 'name' argument is required. The 'regexp' argument is optional: without it the predicate
// only requires the cookie to exist.
func NewCookiePredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		name, err := shared.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		regex, err := optionalRegexpArg(args)
		if err != nil {
			return nil, err
		}
		return NewCookiePredicate(name, regex)
	}
}

// Test checks if the cookie predicate matches the given request.
//
// If the cookie does not exist, the predicate will return false.
// If the cookie exists and there is no regexp, the predicate will return true, even when its value is empty.
// If the cookie exists and there is a regexp, the predicate will return true when its value (which
// can be empty) matches the regexp. A cookie sent more than once matches when at least one of
// its values does.
func (p *Cookie) Test(request *http.Request) bool {
	if !p.mentionsName(request.Header["Cookie"]) {
		return false
	}
	for _, cookie := range request.CookiesNamed(p.name) {
		if p.pattern == nil || p.pattern.MatchString(cookie.Value) {
			return true
		}
	}
	return false
}

// Name returns the name of the predicate.
func (p *Cookie) Name() string {
	return CookiePredicateName
}

// mentionsName reports whether the name of the cookie appears anywhere in the given Cookie header lines.
//
// Cookie names are sent as they are, so a request whose Cookie header does not even contain the
// name cannot carry the cookie. Knowing it beforehand spares parsing the header, which allocates
// regardless of the outcome, for the candidate routes that are not going to match.
func (p *Cookie) mentionsName(lines []string) bool {
	for _, line := range lines {
		if strings.Contains(line, p.name) {
			return true
		}
	}
	return false
}
