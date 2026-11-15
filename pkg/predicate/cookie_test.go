package predicate_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func TestNewCookiePredicateBuilder(t *testing.T) {
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
			name: "build should succeed when cookie regexp argument is not present",
			args: map[string]any{
				"name": "First",
			},
			expectedErr: nil,
		},
		{
			name: "build should succeed when cookie regexp argument is null",
			args: map[string]any{
				"name":   "First",
				"regexp": nil,
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
			name: "build should fail when cookie regexp argument is not a string",
			args: map[string]any{
				"name":   "First",
				"regexp": 1234,
			},
			expectedErr: errors.New("failed to convert 'regexp' attribute: value is required to be a valid string"),
		},
		{
			name: "build should fail when cookie regexp argument is not a valid regexp",
			args: map[string]any{
				"name":   "First",
				"regexp": "[",
			},
			expectedErr: errors.New("invalid cookie regexp: error parsing regexp: missing closing ]: `[`"),
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

func TestNewCookiePredicateBuilder_WithoutRegexp(t *testing.T) {
	tests := []struct {
		name     string
		cookie   string
		expected bool
	}{
		{
			name:     "built predicate should match when cookie is present",
			cookie:   "Second=any2; First=any1",
			expected: true,
		},
		{
			name:     "built predicate shouldn't match when cookie is not present",
			cookie:   "Second=any2",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := predicate.NewCookiePredicateBuilder().Build(map[string]any{"name": "First"})
			if err != nil {
				t.Fatalf("expected err %v actual %v", nil, err)
			}
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test", nil)
			req.Header.Set("Cookie", tt.cookie)

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}

func TestCookiePredicate_Test(t *testing.T) {
	tests := []struct {
		name        string
		cookieName  string
		cookieRegex string
		cookies     []string
		expected    bool
	}{
		{
			name:        "test should match when cookie is present and no regex",
			cookieName:  "First",
			cookieRegex: "",
			cookies:     []string{"First="},
			expected:    true,
		},
		{
			name:        "test shouldn't match when cookie is not present and no regex",
			cookieName:  "Second",
			cookieRegex: "",
			cookies:     []string{"First="},
			expected:    false,
		},
		{
			name:        "test should match when cookie is present and regex matches",
			cookieName:  "First",
			cookieRegex: "any1",
			cookies:     []string{"First=any1"},
			expected:    true,
		},
		{
			name:        "test shouldn't match when cookie is present and regex doesn't match",
			cookieName:  "First",
			cookieRegex: "any1",
			cookies:     []string{"First=any2"},
			expected:    false,
		},
		{
			name:        "test shouldn't match when there is no cookie header and no regex",
			cookieName:  "First",
			cookieRegex: "",
			cookies:     nil,
			expected:    false,
		},
		{
			name:        "test shouldn't match when there is no cookie header and regex matches empty value",
			cookieName:  "First",
			cookieRegex: "^$",
			cookies:     nil,
			expected:    false,
		},
		{
			name:        "test should match when cookie is present with empty value and regex matches empty value",
			cookieName:  "First",
			cookieRegex: "^$",
			cookies:     []string{"Second=any2; First="},
			expected:    true,
		},
		{
			name:        "test shouldn't match when cookie is present with empty value and regex doesn't match empty value",
			cookieName:  "First",
			cookieRegex: "any1",
			cookies:     []string{"First="},
			expected:    false,
		},
		{
			name:        "test should match when cookie is present without equals sign and no regex",
			cookieName:  "First",
			cookieRegex: "",
			cookies:     []string{"Second=any2; First"},
			expected:    true,
		},
		{
			name:        "test should match when cookie is present among others and regex matches",
			cookieName:  "First",
			cookieRegex: "^any1$",
			cookies:     []string{"Second=any2; First=any1; Third=any3"},
			expected:    true,
		},
		{
			name:        "test should match when cookie value is quoted and regex matches unquoted value",
			cookieName:  "First",
			cookieRegex: "^any1$",
			cookies:     []string{`First="any1"`},
			expected:    true,
		},
		{
			name:        "test should match when cookie is sent twice and no regex",
			cookieName:  "First",
			cookieRegex: "",
			cookies:     []string{"First=any2; First=any1"},
			expected:    true,
		},
		{
			name:        "test should match when cookie is sent twice and only the last one matches",
			cookieName:  "First",
			cookieRegex: "any1",
			cookies:     []string{"First=any2; Second=any3; First=any1"},
			expected:    true,
		},
		{
			name:        "test should match when cookie is sent in two cookie headers and only the last one matches",
			cookieName:  "First",
			cookieRegex: "any1",
			cookies:     []string{"First=any2", "Second=any3; First=any1"},
			expected:    true,
		},
		{
			name:        "test shouldn't match when cookie is sent twice and none matches",
			cookieName:  "First",
			cookieRegex: "any1",
			cookies:     []string{"First=any2; First=any3"},
			expected:    false,
		},
		{
			name:        "test shouldn't match when cookie name is only part of another cookie name",
			cookieName:  "First",
			cookieRegex: "",
			cookies:     []string{"FirstOne=any1; MyFirst=any1"},
			expected:    false,
		},
		{
			name:        "test shouldn't match when cookie name is only the value of another cookie",
			cookieName:  "First",
			cookieRegex: "",
			cookies:     []string{"Second=First"},
			expected:    false,
		},
		{
			name:        "test shouldn't match when regex only matches the value of another cookie",
			cookieName:  "First",
			cookieRegex: "any1",
			cookies:     []string{"Second=any1; First=any2"},
			expected:    false,
		},
		{
			name:        "test shouldn't match when cookie name has a different case",
			cookieName:  "First",
			cookieRegex: "",
			cookies:     []string{"first=any1"},
			expected:    false,
		},
		{
			name:        "test shouldn't match when cookie value is not valid",
			cookieName:  "First",
			cookieRegex: "",
			cookies:     []string{`First=any 1"`},
			expected:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := predicate.NewCookiePredicate(tt.cookieName, tt.cookieRegex)
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test", nil)
			for _, cookie := range tt.cookies {
				req.Header.Add("Cookie", cookie)
			}

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}

func TestNewCookiePredicate(t *testing.T) {
	tests := []struct {
		expectedErr error
		name        string
		cookieName  string
		cookieRegex string
	}{
		{
			name:        "test should succeed when cookie is present and no regex",
			cookieName:  "First",
			cookieRegex: "",
			expectedErr: nil,
		},
		{
			name:        "test should succeed when cookie and regex are present",
			cookieName:  "First",
			cookieRegex: "[0-9].*",
			expectedErr: nil,
		},
		{
			name:        "test should return error when regex is not valid",
			cookieName:  "First",
			cookieRegex: "[",
			expectedErr: errors.New("invalid cookie regexp: error parsing regexp: missing closing ]: `[`"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := predicate.NewCookiePredicate(tt.cookieName, tt.cookieRegex)
			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestCookiePredicate_Name(t *testing.T) {
	p, _ := predicate.NewCookiePredicate("SessionId", "1234")

	if p.Name() != predicate.CookiePredicateName {
		t.Errorf("expected %s actual %s", predicate.CookiePredicateName, p.Name())
	}
}
