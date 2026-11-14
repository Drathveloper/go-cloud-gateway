package predicate_test

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func TestFactory_Build(t *testing.T) {
	tests := []struct {
		expected    any
		expectedErr error
		builderArgs map[string]any
		name        string
		builderName string
	}{
		{
			name:        "build should succeed when builder is registered",
			builderName: "Method",
			builderArgs: map[string]any{
				"methods": []string{http.MethodGet, http.MethodPost},
			},
			expected:    predicate.NewMethodPredicate(http.MethodGet, http.MethodPost),
			expectedErr: nil,
		},
		{
			name:        "build should return error when builder failed",
			builderName: "Method",
			builderArgs: map[string]any{},
			expected:    nil,
			expectedErr: errors.New("invalid predicate args: name Method and args map[]: failed to convert 'methods' attribute: value is required"),
		},
		{
			name:        "build should return error with the cause when regexp is not valid",
			builderName: "Cookie",
			builderArgs: map[string]any{
				"name":   "session",
				"regexp": "(",
			},
			expected:    nil,
			expectedErr: errors.New("invalid predicate args: name Cookie and args map[name:session regexp:(]: invalid cookie regexp: error parsing regexp: missing closing ): `(`"),
		},
		{
			name:        "build should return error when builder is not registered",
			builderName: "Invent",
			builderArgs: map[string]any{},
			expected:    nil,
			expectedErr: errors.New("invalid predicate args: name: Invent"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := predicate.NewFactory(predicate.BuilderRegistry)

			actual, err := factory.Build(tt.builderName, tt.builderArgs)

			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if tt.expectedErr != nil && !errors.Is(err, predicate.ErrInvalidPredicate) {
				t.Errorf("expected err %v to wrap %v", err, predicate.ErrInvalidPredicate)
			}
			if !reflect.DeepEqual(tt.expected, actual) {
				t.Errorf("expected %v actual %v", tt.expected, actual)
			}
		})
	}
}

func TestFactory_Build_WrapsBuilderCause(t *testing.T) {
	errCause := errors.New("some cause")
	factory := predicate.NewFactory(gateway.PredicateBuilderRegistry{
		"Failing": gateway.PredicateBuilderFunc(func(_ map[string]any) (gateway.Predicate, error) {
			return nil, errCause
		}),
	})

	actual, err := factory.Build("Failing", map[string]any{"key": "value"})

	if actual != nil {
		t.Errorf("expected %v actual %v", nil, actual)
	}
	if !errors.Is(err, predicate.ErrInvalidPredicate) {
		t.Errorf("expected err %v to wrap %v", err, predicate.ErrInvalidPredicate)
	}
	if !errors.Is(err, errCause) {
		t.Errorf("expected err %v to wrap %v", err, errCause)
	}
	expectedErr := "invalid predicate args: name Failing and args map[key:value]: some cause"
	if expectedErr != fmt.Sprintf("%s", err) {
		t.Errorf("expected err %s actual %s", expectedErr, err)
	}
}
