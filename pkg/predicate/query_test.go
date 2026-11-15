package predicate_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

// maxParsedQueryParams is the number of parameters url.ParseQuery accepts by default: beyond it the
// parsed query is empty.
const maxParsedQueryParams = 10000

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
			name: "build should succeed when regexp argument is not present",
			args: map[string]any{
				"name": "any2",
			},
			expectedErr: nil,
		},
		{
			name: "build should succeed when regexp argument is null",
			args: map[string]any{
				"name":   "any2",
				"regexp": nil,
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
			name: "build should fail when regexp argument is not a string",
			args: map[string]any{
				"name":   "any2",
				"regexp": 1234,
			},
			expectedErr: errors.New("failed to convert 'regexp' attribute: value is required to be a valid string"),
		},
		{
			name: "build should fail when regexp argument is not a valid regexp",
			args: map[string]any{
				"name":   "any2",
				"regexp": "[",
			},
			expectedErr: errors.New("invalid regexp: error parsing regexp: missing closing ]: `[`"),
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

func TestNewQueryPredicateBuilder_WithoutRegexp(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{
			name:     "built predicate should match when query param is present",
			query:    "qp2=abc&qp1=cde",
			expected: true,
		},
		{
			name:     "built predicate shouldn't match when query param is not present",
			query:    "qp2=abc",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := predicate.NewQueryPredicateBuilder().Build(map[string]any{"name": "qp1"})
			if err != nil {
				t.Fatalf("expected err %v actual %v", nil, err)
			}
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test?"+tt.query, nil)

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
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
		{
			name:       "test shouldn't match when there is no query string and no regex present",
			queryParam: "flag",
			regex:      "",
			query:      "",
			expected:   false,
		},
		{
			name:       "test shouldn't match when there is no query string and regex matches empty value",
			queryParam: "flag",
			regex:      "^$",
			query:      "",
			expected:   false,
		},
		{
			name:       "test should match when query param is present with empty value and no regex present",
			queryParam: "flag",
			regex:      "",
			query:      "qp1=cde&flag=",
			expected:   true,
		},
		{
			name:       "test should match when query param is present without value and no regex present",
			queryParam: "flag",
			regex:      "",
			query:      "flag&qp1=cde",
			expected:   true,
		},
		{
			name:       "test should match when query param is present with empty value and regex matches empty value",
			queryParam: "flag",
			regex:      "^$",
			query:      "flag=",
			expected:   true,
		},
		{
			name:       "test should match when query param is present without value and regex matches empty value",
			queryParam: "flag",
			regex:      "^$",
			query:      "flag",
			expected:   true,
		},
		{
			name:       "test shouldn't match when query param is present with empty value and regex doesn't match empty value",
			queryParam: "flag",
			regex:      "1",
			query:      "flag=",
			expected:   false,
		},
		{
			name:       "test should match when query param has several values and no regex present",
			queryParam: "page",
			regex:      "",
			query:      "page=23&page=22",
			expected:   true,
		},
		{
			name:       "test should match when query param has several values and only the last one matches",
			queryParam: "page",
			regex:      "22",
			query:      "page=23&qp1=cde&page=&page=22",
			expected:   true,
		},
		{
			name:       "test shouldn't match when query param has several values and none matches",
			queryParam: "page",
			regex:      "22",
			query:      "page=23&page=24",
			expected:   false,
		},
		{
			name:       "test shouldn't match when query param name is only the prefix of another one",
			queryParam: "page",
			regex:      "",
			query:      "pages=22&mypage=22",
			expected:   false,
		},
		{
			name:       "test shouldn't match when regex only matches the value of another query param",
			queryParam: "page",
			regex:      "22",
			query:      "size=22&page=23",
			expected:   false,
		},
		{
			name:       "test should match when query param name is escaped",
			queryParam: "my page",
			regex:      "22",
			query:      "my%20page=22",
			expected:   true,
		},
		{
			name:       "test should match when query param name has a plus sign in place of a space",
			queryParam: "my page",
			regex:      "22",
			query:      "my+page=22",
			expected:   true,
		},
		{
			name:       "test shouldn't match when query param name has a plus sign and raw name is the same",
			queryParam: "my+page",
			regex:      "",
			query:      "my+page=22",
			expected:   false,
		},
		{
			name:       "test should match when query param value is escaped and regex matches unescaped value",
			queryParam: "sort",
			regex:      "^created,desc by$",
			query:      "sort=created%2Cdesc+by",
			expected:   true,
		},
		{
			name:       "test shouldn't match when query param value has an invalid escape and no regex present",
			queryParam: "flag",
			regex:      "",
			query:      "flag=%zz",
			expected:   false,
		},
		{
			name:       "test should match when query param has an invalid value and then a valid one",
			queryParam: "flag",
			regex:      "",
			query:      "flag=%zz&flag=1",
			expected:   true,
		},
		{
			name:       "test shouldn't match when query param name has an invalid escape",
			queryParam: "flag%zz",
			regex:      "",
			query:      "flag%zz=1",
			expected:   false,
		},
		{
			name:       "test shouldn't match when query param is in a setting with a semicolon",
			queryParam: "flag",
			regex:      "",
			query:      "flag=1;qp1=cde",
			expected:   false,
		},
		{
			name:       "test should match when query string has empty settings",
			queryParam: "flag",
			regex:      "1",
			query:      "&&qp1=cde&&flag=1&",
			expected:   true,
		},
		{
			name:       "test should match when query string has more params than the parsed query allows",
			queryParam: "flag",
			regex:      "1",
			query:      strings.Repeat("qp1=cde&", maxParsedQueryParams) + "flag=1",
			expected:   true,
		},
		{
			name:       "test should match when query param value has an equals sign",
			queryParam: "filter",
			regex:      "^name=abc$",
			query:      "filter=name=abc",
			expected:   true,
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

// The predicate reads the raw query string on its own, so it must keep agreeing with the parsed
// query the rest of the gateway and the backends work with.
func TestQueryPredicate_Test_AgreesWithParsedQuery(t *testing.T) {
	queries := []string{
		"",
		"&",
		"=",
		"=1",
		"a",
		"a=",
		"a=1",
		"a=1&a=2",
		"a=2&a=1",
		"b=1&a=1",
		"b=1",
		"ab=1&ba=1",
		"a=1;b=2",
		"a;=1",
		"a=1&b=2;c=3&a=3",
		"%61=1",
		"%6=1",
		"a=%31",
		"a=%3",
		"a=%zz&a=1",
		"+=1",
		"%20=1",
		"a+=1",
		"a=+",
		"a==1",
		"a=1=1",
		"&&a=1&&",
		"A=1",
		"a=1&%zz",
		"%zz&a=1",
		"a%3D1",
		"a%261=1",
		"a=%26&b=1",
		"á=1",
		"a=á",
	}
	names := []string{"a", "b", " ", "", "A", "a=1", "a&1", "a ", "á"}
	regexps := []string{"", "^$", "1", "^1$", "^[0-9]$", " ", "&", "á"}

	for _, query := range queries {
		for _, name := range names {
			for _, regex := range regexps {
				p, err := predicate.NewQueryPredicate(name, regex)
				if err != nil {
					t.Fatalf("expected err %v actual %v", nil, err)
				}
				req := &http.Request{URL: &url.URL{Path: "/server/test", RawQuery: query}}
				expected := slices.ContainsFunc(req.URL.Query()[name], regexp.MustCompile(regex).MatchString)

				actual := p.Test(req)

				if expected != actual {
					t.Errorf("query %q name %q regexp %q: expected %t actual %t", query, name, regex, expected, actual)
				}
			}
		}
	}
}

func TestQueryPredicate_Name(t *testing.T) {
	p, _ := predicate.NewQueryPredicate("name", "")

	if p.Name() != predicate.QueryPredicateName {
		t.Errorf("expected %s actual %s", predicate.QueryPredicateName, p.Name())
	}
}
