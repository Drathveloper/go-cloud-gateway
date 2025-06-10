package predicate_test

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

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
			expectedErr: errors.New("invalid predicate args: name Method and args map[]"),
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
			if !reflect.DeepEqual(tt.expected, actual) {
				t.Errorf("expected %v actual %v", tt.expected, actual)
			}
		})
	}
}
