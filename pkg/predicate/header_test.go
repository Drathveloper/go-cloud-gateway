package predicate_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func TestNewHeaderPredicateBuilder(t *testing.T) {
	tests := []struct {
		expectedErr error
		args        map[string]any
		name        string
	}{
		{
			name: "build should succeed when args are present and are valid",
			args: map[string]any{
				"name":   "First",
				"regexp": "any1",
			},
			expectedErr: nil,
		},
		{
			name: "build should fail when header name argument is not valid",
			args: map[string]any{
				"regexp": "any1",
			},
			expectedErr: errors.New("failed to convert 'name' attribute: value is required"),
		},
		{
			name: "build should fail when header regexp argument is not valid",
			args: map[string]any{
				"name": "First",
			},
			expectedErr: errors.New("failed to convert 'regexp' attribute: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := predicate.NewHeaderPredicateBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestHeaderPredicate_Test(t *testing.T) {
	tests := []struct {
		header      http.Header
		name        string
		headerName  string
		headerValue string
		expected    bool
	}{
		{
			name:        "test should match when header is present and no value",
			headerName:  "X-Test-Id",
			headerValue: "",
			header:      http.Header{"X-Test-Id": {"666"}},
			expected:    true,
		},
		{
			name:        "test shouldn't match when header is not present and no regex",
			headerName:  "X-Test-Id",
			headerValue: "",
			header:      http.Header{"X-Test-Other": {"666"}},
			expected:    false,
		},
		{
			name:        "test should match when header is present and value matches",
			headerName:  "X-Test-Id",
			headerValue: "1234",
			header:      http.Header{"X-Test-Id": {"1234"}},
			expected:    true,
		},
		{
			name:        "test shouldn't match when header is present and value doesn't match",
			headerName:  "X-Test-Id",
			headerValue: "1234",
			header:      http.Header{"X-Test-Other": {"5678"}},
			expected:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := predicate.NewHeaderPredicate(tt.headerName, tt.headerValue)
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test", nil)
			req.Header = tt.header

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}
