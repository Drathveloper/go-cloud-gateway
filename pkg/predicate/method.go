package predicate

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// MethodPredicateName is the name of the method predicate.
const MethodPredicateName = "Method"

// Method is a predicate that checks if the request method matches a given method.
type Method struct {
	methods []string
}

// NewMethodPredicate creates a new method predicate.
//
// The methods can be written in any case. Requests carry their method in upper case and
// the comparison is exact, so a method configured as "get" would otherwise build a
// predicate that never matches, without any error.
func NewMethodPredicate(methods ...string) *Method {
	upperMethods := make([]string, 0, len(methods))
	for _, method := range methods {
		upperMethods = append(upperMethods, strings.ToUpper(method))
	}
	return &Method{
		methods: upperMethods,
	}
}

// NewMethodPredicateBuilder creates a new method predicate builder.
func NewMethodPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		methods, err := shared.ConvertToStringSlice(args["methods"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'methods' attribute: %w", err)
		}
		return NewMethodPredicate(methods...), nil
	}
}

// Test checks if the request method matches the given method.
//
// If the request method does not match any method, the predicate will return false.
// If the request method matches at least one method, the predicate will return true.
func (p *Method) Test(r *http.Request) bool {
	return slices.Contains(p.methods, r.Method)
}

// Name returns the name of the predicate.
func (p *Method) Name() string {
	return MethodPredicateName
}
