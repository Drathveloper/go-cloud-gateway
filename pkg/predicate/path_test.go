package predicate_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func TestNewPathPredicateBuilder(t *testing.T) {
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
			name:        "build should fail when patterns argument is not valid",
			args:        map[string]any{},
			expectedErr: errors.New("failed to convert 'patterns' attribute: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := predicate.NewPathPredicateBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestPath_Test(t *testing.T) {
	tests := []struct {
		Name          string
		ReqPath       string
		PredicatePath []string
		Expected      bool
	}{
		{
			Name:          "Path should match",
			ReqPath:       "/server/one",
			PredicatePath: []string{"/server/**"},
			Expected:      true,
		},
		{
			Name:          "Path should match one of predicate paths",
			ReqPath:       "/server/one/x/t",
			PredicatePath: []string{"/server/two/**", "/server/one/**"},
			Expected:      true,
		},
		{
			Name:          "Path should not match",
			ReqPath:       "/servor",
			PredicatePath: []string{"/server/**"},
			Expected:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			pred := predicate.NewPathPredicate(tt.PredicatePath...)
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, tt.ReqPath, nil)
			actual := pred.Test(req)
			if tt.Expected != actual {
				t.Errorf("expected %t actual %t", tt.Expected, actual)
			}
		})
	}
}

func TestPath_Name(t *testing.T) {
	expected := "Path"
	pred := predicate.NewPathPredicate("/**")
	if expected != pred.Name() {
		t.Errorf("expected %s actual %s", expected, pred.Name())
	}
}
