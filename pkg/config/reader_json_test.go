package config_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
)

func TestReaderJSON_Read(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *config.Config
		expectedErr error
	}{
		{
			name:  "read should succeed when input is valid",
			input: "{\"gateway\":{\"routes\":[{\"id\":\"someId\",\"uri\":\"someUri\",\"predicates\":[{\"name\":\"Method\",\"args\":{\"methods\":[\"GET\",\"POST\"]}}],\"filters\":[{\"name\":\"AddRequestHeader\",\"args\":{\"name\":\"X-Test\",\"value\":\"True\"}}]}]}}",
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
			input:       "{\"gateway\":{\"routes\":[{\"uri\":\"someUri\",\"predicates\":[{\"name\":\"Method\",\"args\":{\"methods\":[\"GET\",\"POST\"]}}],\"filters\":[{\"name\":\"AddRequestHeader\",\"args\":{\"name\":\"X-Test\",\"value\":\"True\"}}]}]}}",
			expected:    nil,
			expectedErr: errors.New("read json config failed: Key: 'Config.Gateway.Routes[0].ID' Error:Field validation for 'ID' failed on the 'required' tag"),
		},
		{
			name:        "read should return error when json is not well formed",
			input:       "{a:b}",
			expected:    nil,
			expectedErr: errors.New("read json config failed: invalid character 'a' looking for beginning of object key string"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validate := validator.New()
			reader := config.NewReaderJSON(validate)

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
