package predicate

import (
	"fmt"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// MethodPredicateName is the name of the method predicate.
const MethodPredicateName = "Method"

// Method is a predicate that checks if the request method matches a given method.
type Method struct {
	methods []string
}

// NewMethodPredicate creates a new method predicate.
func NewMethodPredicate(methods ...string) *Method {
	return &Method{
		methods: methods,
	}
}

// NewMethodPredicateBuilder creates a new method predicate builder.
func NewMethodPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		methods, err := common.ConvertToStringSlice(args["methods"])
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
	for _, method := range p.methods {
		if r.Method == method {
			return true
		}
	}
	return false
}

func (p *Method) Name() string {
	return MethodPredicateName
}
