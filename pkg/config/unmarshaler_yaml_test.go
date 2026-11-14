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
		expectedErr error
		name        string
		input       string
		expected    config.Duration
	}{
		{
			name:        "unmarshal yaml should succeed when duration is valid string",
			input:       "value: 30s",
			expected:    config.Duration{Duration: 30 * time.Second},
			expectedErr: nil,
		},
		{
			name:        "unmarshal yaml should succeed when duration combines several units",
			input:       "value: 1m30s",
			expected:    config.Duration{Duration: 90 * time.Second},
			expectedErr: nil,
		},
		{
			name:        "unmarshal yaml should succeed when duration is in milliseconds",
			input:       "value: 250ms",
			expected:    config.Duration{Duration: 250 * time.Millisecond},
			expectedErr: nil,
		},
		{
			name:        "unmarshal yaml should keep zero duration when field is absent",
			input:       "other: 1",
			expected:    config.Duration{},
			expectedErr: nil,
		},
		{
			name:        "unmarshal yaml should return error when duration is an integer without unit",
			input:       "value: 30",
			expected:    config.Duration{},
			expectedErr: errors.New("unmarshal duration failed: duration 30 has no unit: use a string with a unit such as \"30s\""),
		},
		{
			name:        "unmarshal yaml should return error when duration is a number of nanoseconds",
			input:       "value: 30000000000",
			expected:    config.Duration{},
			expectedErr: errors.New("unmarshal duration failed: duration 30000000000 has no unit: use a string with a unit such as \"30s\""),
		},
		{
			name:        "unmarshal yaml should return error when duration is a float without unit",
			input:       "value: 30.5",
			expected:    config.Duration{},
			expectedErr: errors.New("unmarshal duration failed: duration 30.5 has no unit: use a string with a unit such as \"30s\""),
		},
		{
			name:        "unmarshal yaml should return error when duration is a string without unit",
			input:       "value: '30'",
			expected:    config.Duration{},
			expectedErr: errors.New("unmarshal duration failed: time: missing unit in duration \"30\""),
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
			if tt.expectedErr != nil && !errors.Is(err, config.ErrUnmarshalDuration) {
				t.Errorf("expected err %v to wrap %v", err, config.ErrUnmarshalDuration)
			}
			if tt.expected != actual.Value {
				t.Errorf("expected %v actual %v", tt.expected, actual.Value)
			}
		})
	}
}
