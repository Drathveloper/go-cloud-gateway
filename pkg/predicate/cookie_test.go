package predicate_test

import (
	"errors"
	"fmt"
	"gateway/pkg/predicate"
	"net/http"
	"testing"
)

func TestNewCookiePredicateBuilder(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		expectedErr error
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
			name: "build should fail when cookie name argument is not valid",
			args: map[string]any{
				"regexp": "any1",
			},
			expectedErr: errors.New("failed to convert 'name' attribute: value is required"),
		},
		{
			name: "build should fail when cookie regexp argument is not valid",
			args: map[string]any{
				"name": "First",
			},
			expectedErr: errors.New("failed to convert 'regexp' attribute: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := predicate.NewCookiePredicateBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestCookiePredicate_Test(t *testing.T) {
	tests := []struct {
		name        string
		cookieName  string
		cookieRegex string
		cookie      *http.Cookie
		expected    bool
	}{
		{
			name:        "test should match when cookie is present and no regex",
			cookieName:  "First",
			cookieRegex: "",
			cookie: &http.Cookie{
				Name: "First",
			},
			expected: true,
		},
		{
			name:        "test shouldn't match when cookie is not present and no regex",
			cookieName:  "Second",
			cookieRegex: "",
			cookie: &http.Cookie{
				Name: "First",
			},
			expected: false,
		},
		{
			name:        "test should match when cookie is present and regex matches",
			cookieName:  "First",
			cookieRegex: "any1",
			cookie: &http.Cookie{
				Name:  "First",
				Value: "any1",
			},
			expected: true,
		},
		{
			name:        "test shouldn't match when cookie is present and regex doesn't match",
			cookieName:  "First",
			cookieRegex: "any1",
			cookie: &http.Cookie{
				Name:  "First",
				Value: "any2",
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := predicate.NewCookiePredicate(tt.cookieName, tt.cookieRegex)
			req, _ := http.NewRequest(http.MethodPost, "/server/test", nil)
			req.AddCookie(tt.cookie)

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}
