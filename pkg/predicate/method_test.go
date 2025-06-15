package predicate_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func TestNewMethodPredicateBuilder(t *testing.T) {
	tests := []struct {
		expectedErr error
		args        map[string]any
		name        string
	}{
		{
			name: "build should succeed when args are present and are valid",
			args: map[string]any{
				"methods": []any{"POST", "GET"},
			},
			expectedErr: nil,
		},
		{
			name:        "build should fail when methods argument is not valid",
			args:        map[string]any{},
			expectedErr: errors.New("failed to convert 'methods' attribute: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := predicate.NewMethodPredicateBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestMethod_Test(t *testing.T) {
	tests := []struct {
		Name             string
		ReqMethod        string
		PredicateMethods []string
		Expected         bool
	}{
		{
			Name:             "Method should match",
			ReqMethod:        http.MethodPost,
			PredicateMethods: []string{http.MethodPost},
			Expected:         true,
		},
		{
			Name:             "Method should match one of predicate methods",
			ReqMethod:        http.MethodPut,
			PredicateMethods: []string{http.MethodPost, http.MethodPut},
			Expected:         true,
		},
		{
			Name:             "Method should not match",
			ReqMethod:        http.MethodPut,
			PredicateMethods: []string{http.MethodPost},
			Expected:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			pred := predicate.NewMethodPredicate(tt.PredicateMethods...)
			req, _ := http.NewRequestWithContext(t.Context(), tt.ReqMethod, "/", nil)
			actual := pred.Test(req)
			if tt.Expected != actual {
				t.Errorf("expected %t actual %t", tt.Expected, actual)
			}
		})
	}
}

func TestMethodPredicate_Name(t *testing.T) {
	p := predicate.NewMethodPredicate(http.MethodPost, http.MethodGet)

	if p.Name() != predicate.MethodPredicateName {
		t.Errorf("expected %s actual %s", predicate.MethodPredicateName, p.Name())
	}
}
