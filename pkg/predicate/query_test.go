package predicate_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func TestNewQueryPredicateBuilder(t *testing.T) {
	tests := []struct {
		expectedErr error
		args        map[string]any
		name        string
	}{
		{
			name: "build should succeed when args are present and are valid",
			args: map[string]any{
				"name":   "any2",
				"regexp": "any1",
			},
			expectedErr: nil,
		},
		{
			name: "build should fail when name argument is not valid",
			args: map[string]any{
				"regexp": "any1",
			},
			expectedErr: errors.New("failed to convert 'name' attribute: value is required"),
		},
		{
			name: "build should fail when regexp argument is not valid",
			args: map[string]any{
				"name": "any2",
			},
			expectedErr: errors.New("failed to convert 'regexp' attribute: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := predicate.NewQueryPredicateBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestQueryPredicate_Test(t *testing.T) {
	tests := []struct {
		name       string
		queryParam string
		regex      string
		query      string
		expected   bool
	}{
		{
			name:       "test should match when query param is present and no regex present",
			queryParam: "qp1",
			regex:      "",
			query:      "qp2=abc&qp1=cde",
			expected:   true,
		},
		{
			name:       "test should match when query param is present and regex present and matches",
			queryParam: "qp1",
			regex:      "cde",
			query:      "qp2=abc&qp1=cde",
			expected:   true,
		},
		{
			name:       "test shouldn't match when query param is not present and no regex present",
			queryParam: "page",
			regex:      "",
			query:      "qp2=abc&qp1=cde",
			expected:   false,
		},
		{
			name:       "test shouldn't match when query param is present and regex present and doesn't match",
			queryParam: "page",
			regex:      "22",
			query:      "qp2=abc&qp1=cde&page=23",
			expected:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := predicate.NewQueryPredicate(tt.queryParam, tt.regex)
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test?"+tt.query, nil)

			actual := p.Test(req)

			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}

func TestQueryPredicate_Name(t *testing.T) {
	p, _ := predicate.NewQueryPredicate("name", "")

	if p.Name() != predicate.QueryPredicateName {
		t.Errorf("expected %s actual %s", predicate.QueryPredicateName, p.Name())
	}
}
