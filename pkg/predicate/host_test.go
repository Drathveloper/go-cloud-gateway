package predicate_test

import (
	"errors"
	"fmt"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
	"net/http"
	"testing"
)

func TestNewHostPredicateBuilder(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		expectedErr error
	}{
		{
			name: "build should succeed when args are present and are valid",
			args: map[string]any{
				"patterns": []any{"/*", "/**"},
			},
			expectedErr: nil,
		},
		{
			name:        "build should fail when host patterns argument is not valid",
			args:        map[string]any{},
			expectedErr: errors.New("failed to convert 'patterns' attribute: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := predicate.NewHostPredicateBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestHostPredicate_Test(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		host     string
		expected bool
	}{
		{
			name:     "test should match when patterns match host",
			patterns: []string{"**.x.com", "**.example.org"},
			host:     "test.example.org",
			expected: true,
		},
		{
			name:     "test shouldn't match when patterns don't match host",
			patterns: []string{"**.example.org"},
			host:     "test.other.org",
			expected: false,
		},
		{
			name:     "test shouldn't match when no patterns",
			patterns: []string{},
			host:     "test.other.org",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := predicate.NewHostPredicate(tt.patterns...)
			req, _ := http.NewRequest(http.MethodPost, "/server/test", nil)
			req.Host = tt.host

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}
