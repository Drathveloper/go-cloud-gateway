package predicate_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

type DummyTime struct {
	fixedTime time.Time
}

func (t *DummyTime) Now() time.Time {
	return t.fixedTime
}

func TestBeforePredicate_Test(t *testing.T) {
	tests := []struct {
		name     string
		before   time.Time
		now      time.Time
		expected bool
	}{
		{
			name:     "test before given date should return true when predicate datetime is after now datetime",
			before:   time.Now(),
			now:      time.Now().Add(-100 * time.Second),
			expected: true,
		},
		{
			name:     "test before given date should return false when predicate datetime is before now datetime",
			before:   time.Now().Add(-100 * time.Second),
			now:      time.Now(),
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := predicate.NewBeforePredicateTest(tt.before, &DummyTime{tt.now})
			req, _ := http.NewRequest(http.MethodPost, "/server/test", nil)

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}

func TestNewBeforePredicateBuilder(t *testing.T) {
	datetime := time.Now().Add(-100 * time.Second)
	tests := []struct {
		name        string
		args        map[string]any
		expectedErr error
	}{
		{
			name: "build should succeed when args are present and are valid",
			args: map[string]any{
				"datetime": datetime.Format(time.RFC3339),
			},
			expectedErr: nil,
		},
		{
			name:        "build fail when datetime argument is not valid",
			args:        map[string]any{},
			expectedErr: errors.New("arg 'datetime' is required to be a valid datetime: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := predicate.NewBeforePredicateBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestAfterPredicate_Test(t *testing.T) {
	tests := []struct {
		name     string
		before   time.Time
		now      time.Time
		expected bool
	}{
		{
			name:     "test after given date should return true when predicate datetime is before now datetime",
			before:   time.Now(),
			now:      time.Now().Add(100 * time.Second),
			expected: true,
		},
		{
			name:     "test after given date should return false when predicate datetime is after now datetime",
			before:   time.Now().Add(100 * time.Second),
			now:      time.Now(),
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := predicate.NewAfterPredicateTest(tt.before, &DummyTime{tt.now})
			req, _ := http.NewRequest(http.MethodPost, "/server/test", nil)

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}

func TestNewAfterPredicateBuilder(t *testing.T) {
	datetime := time.Now().Add(-100 * time.Second)
	tests := []struct {
		name        string
		args        map[string]any
		expectedErr error
	}{
		{
			name: "build should succeed when args are present and are valid",
			args: map[string]any{
				"datetime": datetime.Format(time.RFC3339),
			},
			expectedErr: nil,
		},
		{
			name:        "build fail when datetime argument is not valid",
			args:        map[string]any{},
			expectedErr: errors.New("arg 'datetime' is required to be a valid datetime: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := predicate.NewAfterPredicateBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestNewBetweenPredicateBuilder(t *testing.T) {
	before := time.Now().Add(-100 * time.Second)
	after := time.Now().Add(100 * time.Second)
	tests := []struct {
		name        string
		args        map[string]any
		expectedErr error
	}{
		{
			name: "build should succeed when args are present and are valid",
			args: map[string]any{
				"start": before.Format(time.RFC3339),
				"end":   after.Format(time.RFC3339),
			},
			expectedErr: nil,
		},
		{
			name: "build fail when start argument is not valid",
			args: map[string]any{
				"end": after.Format(time.RFC3339),
			},
			expectedErr: errors.New("arg 'start' is required to be a valid datetime: value is required"),
		},
		{
			name: "build fail when end argument is not valid",
			args: map[string]any{
				"start": before.Format(time.RFC3339),
			},
			expectedErr: errors.New("arg 'end' is required to be a valid datetime: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := predicate.NewBetweenPredicateBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestBetweenPredicate_Test(t *testing.T) {
	tests := []struct {
		name     string
		before   time.Time
		after    time.Time
		now      time.Time
		expected bool
	}{
		{
			name:     "test between given date should return true when predicate datetime is between datetime range",
			before:   time.Now().Add(-100 * time.Second),
			after:    time.Now().Add(100 * time.Second),
			now:      time.Now(),
			expected: true,
		},
		{
			name:     "test between given date should return true when predicate datetime is before datetime range",
			before:   time.Now().Add(-100 * time.Second),
			after:    time.Now().Add(100 * time.Second),
			now:      time.Now().Add(-150 * time.Second),
			expected: false,
		},
		{
			name:     "test between given date should return true when predicate datetime is after datetime range",
			before:   time.Now().Add(-100 * time.Second),
			after:    time.Now().Add(100 * time.Second),
			now:      time.Now().Add(150 * time.Second),
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := predicate.NewBetweenPredicateTest(tt.before, tt.after, &DummyTime{tt.now})
			req, _ := http.NewRequest(http.MethodPost, "/server/test", nil)

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}
