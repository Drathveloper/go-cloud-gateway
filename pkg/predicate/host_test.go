package predicate_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func TestNewHostPredicateBuilder(t *testing.T) {
	tests := []struct {
		expectedErr error
		args        map[string]any
		name        string
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
		host     string
		patterns []string
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
		{
			name:     "test should match when host has a port and pattern has none",
			patterns: []string{"example.org"},
			host:     "example.org:8080",
			expected: true,
		},
		{
			name:     "test should match when host and pattern have the same port",
			patterns: []string{"example.org:8080"},
			host:     "example.org:8080",
			expected: true,
		},
		{
			name:     "test shouldn't match when pattern has a port and host has another",
			patterns: []string{"example.org:8080"},
			host:     "example.org:9090",
			expected: false,
		},
		{
			name:     "test should match when host case differs from pattern",
			patterns: []string{"**.example.org"},
			host:     "Test.Example.ORG",
			expected: true,
		},
		{
			name:     "test should match any host when pattern is **",
			patterns: []string{"**"},
			host:     "test.other.org:8080",
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := predicate.NewHostPredicate(tt.patterns...)
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test", nil)
			req.Host = tt.host

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}

func TestHostPredicate_Name(t *testing.T) {
	p, _ := predicate.NewHostPredicate("example.org")

	if p.Name() != predicate.HostPredicateName {
		t.Errorf("expected %s actual %s", predicate.HostPredicateName, p.Name())
	}
}
