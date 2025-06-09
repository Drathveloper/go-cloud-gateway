package config_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
)

type DummyYAML struct {
	Value config.Duration `json:"value" yaml:"value"`
}

func TestDuration_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    config.Duration
		expectedErr error
	}{
		{
			name:        "unmarshal yaml should succeed when duration is valid string",
			input:       "value: 30s",
			expected:    config.Duration{Duration: 30 * time.Second},
			expectedErr: nil,
		},
		{
			name:        "unmarshal yaml should succeed when duration is valid float64",
			input:       "value: 30000000000",
			expected:    config.Duration{Duration: 30 * time.Second},
			expectedErr: nil,
		},
		{
			name:        "unmarshal yaml should return error when duration is invalid string",
			input:       "value: hey",
			expected:    config.Duration{},
			expectedErr: errors.New("unmarshal duration failed: time: invalid duration \"hey\""),
		},
		{
			name:        "unmarshal yaml should return error when duration is invalid type",
			input:       "value: false",
			expected:    config.Duration{},
			expectedErr: errors.New("unmarshal duration failed: invalid duration: false"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actual DummyYAML
			err := yaml.Unmarshal([]byte(tt.input), &actual)

			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if tt.expected != actual.Value {
				t.Errorf("expected %v actual %v", tt.expected, actual.Value)
			}
		})
	}
}
