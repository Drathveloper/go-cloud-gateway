package gateway_test

import (
	"gateway/pkg/gateway"
	"net/http"
	"testing"
)

type DummyPredicate struct {
	Matches bool
}

func (d DummyPredicate) Test(_ *http.Request) bool {
	return d.Matches
}

func TestPredicates_TestAll(t *testing.T) {
	tests := []struct {
		name       string
		predicates []gateway.Predicate
		expected   bool
	}{
		{
			name: "test all should return true when all predicates match",
			predicates: []gateway.Predicate{
				&DummyPredicate{true},
				&DummyPredicate{true},
				&DummyPredicate{true},
			},
			expected: true,
		},
		{
			name: "test all should return false when first predicate doesn't match",
			predicates: []gateway.Predicate{
				&DummyPredicate{false},
				&DummyPredicate{true},
				&DummyPredicate{true},
			},
			expected: false,
		},
		{
			name: "test all should return false when last predicate doesn't match",
			predicates: []gateway.Predicate{
				&DummyPredicate{true},
				&DummyPredicate{true},
				&DummyPredicate{false},
			},
			expected: false,
		},
		{
			name: "test all should return false when none predicate match",
			predicates: []gateway.Predicate{
				&DummyPredicate{false},
				&DummyPredicate{false},
				&DummyPredicate{false},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicates := gateway.Predicates(tt.predicates)
			req := &http.Request{}
			actual := predicates.TestAll(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}
