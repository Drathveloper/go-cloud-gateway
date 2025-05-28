package predicate

import (
	"fmt"
	"net/http"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

const MethodPredicateName = "Method"

type Method struct {
	methods []string
}

func NewMethodPredicate(methods ...string) *Method {
	return &Method{
		methods: methods,
	}
}

func NewMethodPredicateBuilder() gateway.PredicateBuilder {
	return gateway.PredicateBuilderFunc(func(args map[string]any) (gateway.Predicate, error) {
		methods, err := common.ConvertToStringSlice(args["methods"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'methods' attribute: %w", err)
		}
		return NewMethodPredicate(methods...), nil
	})
}

func (m *Method) Test(r *http.Request) bool {
	for _, method := range m.methods {
		if r.Method == method {
			return true
		}
	}
	return false
}

func (m *Method) Name() string {
	return MethodPredicateName
}
