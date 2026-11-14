package filter_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
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
			builderName: "AddRequestHeader",
			builderArgs: map[string]any{
				"name":  "X-Test",
				"value": "True",
			},
			expected:    filter.NewAddRequestHeaderFilter("X-Test", "True"),
			expectedErr: nil,
		},
		{
			name:        "build should return error when builder failed",
			builderName: "AddRequestHeader",
			builderArgs: map[string]any{
				"name": "X-Test",
			},
			expected:    nil,
			expectedErr: errors.New("filter builder failed: filter AddRequestHeader and args map[name:X-Test]: failed to convert 'value' attribute: value is required"),
		},
		{
			name:        "build should return error with the cause when regexp is not valid",
			builderName: "RewritePath",
			builderArgs: map[string]any{
				"regexp":      "(",
				"replacement": "/x",
			},
			expected:    nil,
			expectedErr: errors.New("filter builder failed: filter RewritePath and args map[regexp:( replacement:/x]: failed to build rewrite path filter: error parsing regexp: missing closing ): `(`"),
		},
		{
			name:        "build should return error when builder is not registered",
			builderName: "Invent",
			builderArgs: map[string]any{},
			expected:    nil,
			expectedErr: errors.New("filter builder failed: filter builder not found for filter Invent"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := filter.NewFactory(filter.BuilderRegistry)

			actual, err := factory.Build(tt.builderName, tt.builderArgs)

			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if tt.expectedErr != nil && !errors.Is(err, filter.ErrFilterBuilder) {
				t.Errorf("expected err %v to wrap %v", err, filter.ErrFilterBuilder)
			}
			if !reflect.DeepEqual(tt.expected, actual) {
				t.Errorf("expected %v actual %v", tt.expected, actual)
			}
		})
	}
}

func TestFactory_Build_WrapsBuilderCause(t *testing.T) {
	errCause := errors.New("some cause")
	factory := filter.NewFactory(gateway.FilterBuilderRegistry{
		"Failing": gateway.FilterBuilderFunc(func(_ map[string]any) (gateway.Filter, error) {
			return nil, errCause
		}),
	})

	actual, err := factory.Build("Failing", map[string]any{"key": "value"})

	if actual != nil {
		t.Errorf("expected %v actual %v", nil, actual)
	}
	if !errors.Is(err, filter.ErrFilterBuilder) {
		t.Errorf("expected err %v to wrap %v", err, filter.ErrFilterBuilder)
	}
	if !errors.Is(err, errCause) {
		t.Errorf("expected err %v to wrap %v", err, errCause)
	}
	expectedErr := "filter builder failed: filter Failing and args map[key:value]: some cause"
	if expectedErr != fmt.Sprintf("%s", err) {
		t.Errorf("expected err %s actual %s", expectedErr, err)
	}
}
