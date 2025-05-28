package config_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
)

func TestReaderYAML_Read(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *config.Config
		expectedErr error
	}{
		{
			name:  "read should succeed when input is valid",
			input: "gateway:\n  routes:\n  - id: someId\n    uri: someUri\n    predicates:\n    - name: Method\n      args:\n        methods:\n        - GET\n        - POST\n    filters:\n    - name: AddRequestHeader\n      args:\n        name: X-Test\n        value: 'True'",
			expected: &config.Config{
				Gateway: config.Gateway{
					Routes: []config.Route{
						{
							ID:  "someId",
							URI: "someUri",
							Predicates: []config.ParameterizedItem{
								{
									Name: "Method",
									Args: map[string]any{
										"methods": []any{"GET", "POST"},
									},
								},
							},
							Filters: []config.ParameterizedItem{
								{
									Name: "AddRequestHeader",
									Args: map[string]any{
										"name":  "X-Test",
										"value": "True",
									},
								},
							},
							Timeout: 0,
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name:        "read should return error when json validation failed",
			input:       "gateway:\n  routes:\n  - uri: someUri\n    predicates:\n    - name: Method\n      args:\n        methods:\n        - GET\n        - POST\n    filters:\n    - name: AddRequestHeader\n      args:\n        name: X-Test\n        value: 'True'",
			expected:    nil,
			expectedErr: errors.New("read yaml config failed: Key: 'Config.Gateway.Routes[0].ID' Error:Field validation for 'ID' failed on the 'required' tag"),
		},
		{
			name:        "read should return error when yaml is not well formed",
			input:       "[",
			expected:    nil,
			expectedErr: errors.New("read yaml config failed: yaml: line 1: did not find expected node content"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validate := validator.New()
			reader := config.NewReaderYAML(validate)

			cfg, err := reader.Read([]byte(tt.input))

			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected %s actual %s", tt.expectedErr, err)
			}
			if !reflect.DeepEqual(tt.expected, cfg) {
				t.Errorf("expected %v actual %v", tt.expected, cfg)
			}
		})
	}
}
