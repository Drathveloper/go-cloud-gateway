package predicate

import (
	"fmt"
	"github.com/drathveloper/go-cloud-gateway/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"net/http"
	"regexp"
)

const CookiePredicateName = "Cookie"

type CookiePredicate struct {
	Name    string
	Pattern *regexp.Regexp
}

func NewCookiePredicate(name, regexpStr string) (*CookiePredicate, error) {
	var pattern *regexp.Regexp
	var err error
	if regexpStr != "" {
		pattern, err = regexp.Compile(regexpStr)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp: %v", err)
		}
	}
	return &CookiePredicate{
		Name:    name,
		Pattern: pattern,
	}, nil
}

func NewCookiePredicateBuilder() gateway.PredicateBuilder {
	return gateway.PredicateBuilderFunc(func(args map[string]any) (gateway.Predicate, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		regex, err := common.ConvertToString(args["regexp"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'regexp' attribute: %w", err)
		}
		return NewCookiePredicate(name, regex)
	})
}

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
