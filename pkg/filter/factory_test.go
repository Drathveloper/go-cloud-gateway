package filter_test

import (
	"errors"
	"fmt"
	"gateway/pkg/filter"
	"reflect"
	"testing"
)

func TestFactory_Build(t *testing.T) {
	tests := []struct {
		name        string
		builderName string
		builderArgs map[string]any
		expected    any
		expectedErr error
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
			expectedErr: errors.New("filter builder failed for filter AddRequestHeader and args map[name:X-Test]"),
		},
		{
			name:        "build should return error when builder is not registered",
			builderName: "Invent",
			builderArgs: map[string]any{},
			expected:    nil,
			expectedErr: errors.New("filter builder not found for filter Invent"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := filter.NewFactory(filter.BuilderRegistry)

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
