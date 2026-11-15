package predicate

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
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
// An empty regexp only requires the query parameter to exist.
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
//
// The 'name' argument is required. The 'regexp' argument is optional: without it the predicate
// only requires the query parameter to exist.
func NewQueryPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		name, err := shared.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		regex, err := optionalRegexpArg(args)
		if err != nil {
			return nil, err
		}
		return NewQueryPredicate(name, regex)
	}
}

// Test checks if the query predicate matches the given request.
//
// If the query parameter does not exist, the predicate will return false.
// If the query parameter exists and there is no regexp, the predicate will return true, even when
// it has no value ("?flag" or "?flag=").
// If the query parameter exists and there is a regexp, the predicate will return true when at
// least one of its values (which can be empty) matches the regexp.
//
// The query string is read with the rules of url.ParseQuery: names and values are unescaped, and
// a setting that has an invalid escape or a raw semicolon is ignored. The only difference is that
// the number of parameters is not limited, because none of them is stored.
func (p *Query) Test(request *http.Request) bool {
	// The query string is scanned instead of parsed with request.URL.Query(): this runs for every
	// candidate route of every request, and parsing builds a map with all the parameters only to
	// read one of them.
	rest := request.URL.RawQuery
	for rest != "" {
		var setting string
		setting, rest, _ = strings.Cut(rest, "&")
		if setting == "" || strings.Contains(setting, ";") {
			continue
		}
		key, value, _ := strings.Cut(setting, "=")
		if !p.isName(key) {
			continue
		}
		value, err := url.QueryUnescape(value)
		if err != nil {
			continue
		}
		if p.pattern == nil || p.pattern.MatchString(value) {
			return true
		}
	}
	return false
}

// Name returns the name of the predicate.
func (p *Query) Name() string {
	return QueryPredicateName
}

// isName reports whether the given key of the raw query string is the name of the parameter.
func (p *Query) isName(key string) bool {
	// unescaping only allocates when the key has escapes, which is not the usual case
	name, err := url.QueryUnescape(key)
	return err == nil && name == p.name
}
