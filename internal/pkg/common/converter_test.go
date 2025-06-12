package common_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
)

func TestConvertToString(t *testing.T) {
	tests := []struct {
		input       any
		expectedErr error
		name        string
		expected    string
	}{
		{
			name:        "convert string to string should succeed",
			input:       "someStr",
			expected:    "someStr",
			expectedErr: nil,
		},
		{
			name:        "convert nil to string should return error",
			input:       nil,
			expected:    "",
			expectedErr: errors.New("value is required"),
		},
		{
			name:        "convert other type to string should return error",
			input:       160,
			expected:    "",
			expectedErr: errors.New("value is required to be a valid string"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := common.ConvertToString(tt.input)

			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected %s actual %s", tt.expectedErr, err)
			}
			if tt.expected != result {
				t.Errorf("expected %s actual %s", tt.expected, result)
			}
		})
	}
}

func TestConvertToStringSlice(t *testing.T) {
	tests := []struct {
		input       any
		expectedErr error
		name        string
		expected    []string
	}{
		{
			name:        "convert any string slice to string slice should succeed",
			input:       []any{"any1", "any2"},
			expected:    []string{"any1", "any2"},
			expectedErr: nil,
		},
		{
			name:        "convert nil to string slice should return error",
			input:       nil,
			expected:    nil,
			expectedErr: errors.New("value is required"),
		},
		{
			name:        "convert string slice to string slice should succeed",
			input:       []string{"any1", "any2"},
			expected:    []string{"any1", "any2"},
			expectedErr: nil,
		},
		{
			name:        "convert mixed slice to string slice should return error",
			input:       []any{"any1", 123},
			expected:    nil,
			expectedErr: errors.New("value is expected to be a valid string slice: value is required to be a valid slice: element at index 1 is not of expected type"),
		},
		{
			name:        "convert other type to string should return error",
			input:       160,
			expected:    nil,
			expectedErr: errors.New("value is required to be a valid slice"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := common.ConvertToStringSlice(tt.input)

			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected %s actual %s", tt.expectedErr, err)
			}
			if !slices.Equal(tt.expected, result) {
				t.Errorf("expected %s actual %s", tt.expected, result)
			}
		})
	}
}

func TestConvertToDateTime(t *testing.T) {
	now := time.Now()
	nowStr := now.Format(time.RFC3339)
	nowFromStr, _ := time.Parse(time.RFC3339, nowStr)
	tests := []struct {
		expected    time.Time
		input       any
		expectedErr error
		name        string
	}{
		{
			name:        "convert datetime to datetime should succeed",
			input:       now,
			expected:    now,
			expectedErr: nil,
		},
		{
			name:        "convert string to datetime should succeed",
			input:       nowStr,
			expected:    nowFromStr,
			expectedErr: nil,
		},
		{
			name:        "convert nil to datetime should return error",
			input:       nil,
			expected:    time.Time{},
			expectedErr: errors.New("value is required"),
		},
		{
			name:        "convert invalid str to datetime should return error",
			input:       "abc",
			expected:    time.Time{},
			expectedErr: errors.New("value is required to be a valid datetime: parsing time \"abc\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"abc\" as \"2006\""),
		},
		{
			name:        "convert other type to datetime should return error",
			input:       false,
			expected:    time.Time{},
			expectedErr: errors.New("value is required to be a valid string"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := common.ConvertToDateTime(tt.input)

			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if !reflect.DeepEqual(tt.expected, result) {
				t.Errorf("expected %s actual %s", tt.expected, result)
			}
		})
	}
}

func TestConvertToInt(t *testing.T) {
	tests := []struct {
		input       any
		expectedErr error
		name        string
		expected    int
	}{
		{
			name:        "convert int to int should succeed",
			input:       160,
			expected:    160,
			expectedErr: nil,
		},
		{
			name:        "convert string to int should succeed",
			input:       "160",
			expected:    160,
			expectedErr: nil,
		},
		{
			name:        "convert nil to int should return error",
			input:       nil,
			expected:    0,
			expectedErr: errors.New("value is required"),
		},
		{
			name:        "convert bool to int should return error",
			input:       false,
			expected:    0,
			expectedErr: errors.New("value is required to be a valid int"),
		},
		{
			name:        "convert non integer string to int should return error",
			input:       "potato",
			expected:    0,
			expectedErr: errors.New("value is required to be a valid int"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := common.ConvertToInt(tt.input)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if tt.expected != result {
				t.Errorf("expected %d actual %d", tt.expected, result)
			}
		})
	}
}
