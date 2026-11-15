package config_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
)

func TestReaderJSON_Read(t *testing.T) {
	tests := []struct {
		expectedErr error
		expected    *config.Config
		name        string
		input       string
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
							Timeout: config.Duration{},
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
			name:        "read should return error when a duration has no unit",
			input:       "{\"gateway\":{\"routes\":[{\"id\":\"someId\",\"uri\":\"http://localhost:8001\",\"timeout\":30}]}}",
			expected:    nil,
			expectedErr: errors.New("read json config failed: unmarshal duration failed: duration 30 has no unit: use a string with a unit such as \"30s\""),
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

func TestReaderJSON_Read_StreamingRoute(t *testing.T) {
	input := `{"gateway":{"routes":[
		{"id":"r1","uri":"http://localhost:8001","streaming":true},
		{"id":"r2","uri":"http://localhost:8001"}]}}`

	cfg, err := config.NewReaderJSON(validator.New()).Read([]byte(input))
	if err != nil {
		t.Fatalf("expected nil err actual %v", err)
	}

	if !cfg.Gateway.Routes[0].Streaming {
		t.Error("expected route r1 to be a streaming route")
	}
	if cfg.Gateway.Routes[1].Streaming {
		t.Error("expected route r2 to be a regular route")
	}
}

func TestReaderJSON_Read_CircuitBreakerFailureRateThreshold(t *testing.T) {
	tests := []struct {
		name      string
		threshold int
		enabled   bool
		expectErr bool
	}{
		{name: "read should succeed when threshold is the lowest percentage", enabled: true, threshold: 1},
		{name: "read should succeed when threshold is the highest percentage", enabled: true, threshold: 100},
		{name: "read should fail when threshold is zero", enabled: true, threshold: 0, expectErr: true},
		{name: "read should fail when threshold is over 100", enabled: true, threshold: 101, expectErr: true},
		{name: "read should fail when threshold is negative", enabled: true, threshold: -1, expectErr: true},
		{name: "read should succeed when the breaker is disabled and has no threshold", enabled: false, threshold: 0},
		{
			name:      "read should fail when the breaker is disabled and threshold is over 100",
			enabled:   false,
			threshold: 101,
			expectErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf(`{"gateway":{"routes":[{"id":"r1","uri":"http://localhost:8001","circuit-breaker":{`+
				`"enabled":%t,"interval":"1m","min-requests-threshold":2,"failure-rate-threshold":%d,`+
				`"wait-duration-in-open-state":"1m","num-allowed-half-open-calls":1}}]}}`, tt.enabled, tt.threshold)

			_, err := config.NewReaderJSON(validator.New()).Read([]byte(input))

			if tt.expectErr != (err != nil) {
				t.Errorf("expected error %t actual %v", tt.expectErr, err)
			}
			if err != nil && !strings.Contains(err.Error(), "FailureRateThreshold") {
				t.Errorf("expected the error to name the threshold, actual %v", err)
			}
		})
	}
}
