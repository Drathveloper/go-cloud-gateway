package predicate_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func TestNewPathPredicateBuilder(t *testing.T) {
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
		{
			Name:          "Path should match when ** has to skip an earlier occurrence of the next segment",
			ReqPath:       "/api/v1/users/extra/users/7",
			PredicatePath: []string{"/api/**/users/*"},
			Expected:      true,
		},
		{
			Name:          "Path should not match when there are no patterns",
			ReqPath:       "/server/one",
			PredicatePath: []string{},
			Expected:      false,
		},
		{
			Name:          "Path should match a literal pattern when a character of it comes encoded",
			ReqPath:       "/files/my%20file",
			PredicatePath: []string{"/files/my file"},
			Expected:      true,
		},
		{
			Name:          "Path should match a literal pattern when an unreserved character comes encoded",
			ReqPath:       "/files/%6Dy%20file",
			PredicatePath: []string{"/files/my file"},
			Expected:      true,
		},
		{
			Name:          "Path should match when the encoded separator is inside the segment of the wildcard",
			ReqPath:       "/static/a%2Fb.css",
			PredicatePath: []string{"/static/*.css"},
			Expected:      true,
		},
		{
			Name:          "Path should match when the encoded separator is in lower case",
			ReqPath:       "/static/a%2fb.css",
			PredicatePath: []string{"/static/*.css"},
			Expected:      true,
		},
		{
			Name:          "Path should not match a pattern with one more segment when the separator is encoded",
			ReqPath:       "/static/a%2Fb.css",
			PredicatePath: []string{"/static/*/b.css"},
			Expected:      false,
		},
		{
			Name:          "Path should not match a literal pattern with one more segment when the separator is encoded",
			ReqPath:       "/static/a%2Fb.css",
			PredicatePath: []string{"/static/a/b.css"},
			Expected:      false,
		},
		{
			Name:          "Path should match one of predicate paths when the separator is encoded",
			ReqPath:       "/static/a%2Fb.css",
			PredicatePath: []string{"/static/*/b.css", "/static/a?b.css"},
			Expected:      true,
		},
		{
			Name:          "Path should match a recursive pattern when the separator is encoded",
			ReqPath:       "/static/a%2Fb.css",
			PredicatePath: []string{"/static/**"},
			Expected:      true,
		},
		{
			Name:          "Path should match when another character comes encoded next to the encoded separator",
			ReqPath:       "/static/my%20a%2Fb.css",
			PredicatePath: []string{"/static/my a?b.css"},
			Expected:      true,
		},
		{
			Name:          "Path should not take an encoded percent sign followed by 2F as an encoded separator",
			ReqPath:       "/static/a%252Fb.css",
			PredicatePath: []string{"/static/a%2Fb.css"},
			Expected:      true,
		},
		{
			Name:          "Path should keep an encoded dot segment and its separator in one segment",
			ReqPath:       "/public/..%2Fadmin",
			PredicatePath: []string{"/public/*/admin", "/admin"},
			Expected:      false,
		},
		{
			Name:          "Path should not match when there are no patterns and the separator is encoded",
			ReqPath:       "/static/a%2Fb.css",
			PredicatePath: []string{},
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

// TestPath_Test_EscapedPath covers the urls net/url does not build out of a request
// target: what the predicate reads is what url.URL.EscapedPath reports, which is also
// what the backend receives.
func TestPath_Test_EscapedPath(t *testing.T) {
	tests := []struct {
		URL           *url.URL
		Name          string
		PredicatePath []string
		Expected      bool
	}{
		{
			Name:          "Path should ignore a raw path that is not an encoding of the path",
			URL:           &url.URL{Path: "/static/a/b.css", RawPath: "/static/x%2Fb.css"},
			PredicatePath: []string{"/static/*.css"},
			Expected:      false,
		},
		{
			Name:          "Path should match the path when the raw path is not an encoding of it",
			URL:           &url.URL{Path: "/static/a/b.css", RawPath: "/static/x%2Fb.css"},
			PredicatePath: []string{"/static/*/b.css"},
			Expected:      true,
		},
		{
			// net/url sets this raw path for the request target /static/a"b%2Fc.css, but
			// EscapedPath rejects it because of the quote and encodes the path again.
			Name:          "Path should ignore a raw path with a character that has to be encoded",
			URL:           &url.URL{Path: `/static/a"b/c.css`, RawPath: `/static/a"b%2Fc.css`},
			PredicatePath: []string{"/static/*.css"},
			Expected:      false,
		},
		{
			Name:          "Path should match the path when the raw path has a character that has to be encoded",
			URL:           &url.URL{Path: `/static/a"b/c.css`, RawPath: `/static/a"b%2Fc.css`},
			PredicatePath: []string{"/static/*/c.css"},
			Expected:      true,
		},
		{
			Name:          "Path should match the raw path when it is an encoding of the path",
			URL:           &url.URL{Path: "/static/a/b.css", RawPath: "/static/a%2Fb.css"},
			PredicatePath: []string{"/static/*.css"},
			Expected:      true,
		},
		{
			Name:          "Path should match the path when there is no raw path",
			URL:           &url.URL{Path: "/static/a/b.css"},
			PredicatePath: []string{"/static/*/b.css"},
			Expected:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			pred := predicate.NewPathPredicate(tt.PredicatePath...)
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
			req.URL = tt.URL
			actual := pred.Test(req)
			if tt.Expected != actual {
				t.Errorf("expected %t actual %t", tt.Expected, actual)
			}
		})
	}
}

// escapeTarget percent-encodes the bytes of the decoded path that shouldEscape picks,
// never a separator and always a percent sign, in the hex digits given.
func escapeTarget(path, hex string, shouldEscape func(index int) bool) string {
	var escaped strings.Builder
	for i := range len(path) {
		char := path[i]
		if char == '/' || (char != '%' && !shouldEscape(i)) {
			escaped.WriteByte(char)
			continue
		}
		escaped.WriteByte('%')
		escaped.WriteByte(hex[char>>4])
		escaped.WriteByte(hex[char&0x0f])
	}
	return escaped.String()
}

// joinPathSegments returns every way of joining up to maxSegments of the given segments.
func joinPathSegments(segments []string, maxSegments int) []string {
	total, levelSize := 0, 1
	for range maxSegments {
		levelSize *= len(segments)
		total += levelSize
	}
	// The empty string at the start is the prefix of the paths of one segment: it is
	// dropped at the end.
	joined := make([]string, 1, total+1)
	levelStart := 0
	for range maxSegments {
		levelEnd := len(joined)
		for _, prefix := range joined[levelStart:levelEnd] {
			for _, segment := range segments {
				joined = append(joined, prefix+"/"+segment)
			}
		}
		levelStart = levelEnd
	}
	return joined[1:]
}

// TestPath_Test_SameAsDecodedPathWithoutEncodedSeparator checks that nothing changes for
// a request without an encoded separator, whatever else it encodes: the predicate reports
// what the pattern reports for the decoded path, which is all it looked at until v1.7.0.
//
// The request targets are parsed the way the server parses them. Each path is sent in the
// encoding net/url would give it, which leaves no raw path, and in others, which set it.
func TestPath_Test_SameAsDecodedPathWithoutEncodedSeparator(t *testing.T) {
	const upperHex, lowerHex = "0123456789ABCDEF", "0123456789abcdef"
	patterns := joinPathSegments([]string{"a", "a b", "*", "?", "**", "a*", "*.c"}, 3)
	patterns = append(patterns, "", "/", "**", "a/b/")
	paths := joinPathSegments([]string{"a", "ab", "", "a b", "a.c", "a%b", "é"}, 3)
	paths = append(paths, "/", "//", "/a/", "/a/b//")

	predicates := make([]*predicate.Path, 0, len(patterns))
	compiled := make([]shared.PathPattern, 0, len(patterns))
	for _, pattern := range patterns {
		predicates = append(predicates, predicate.NewPathPredicate(pattern))
		compiled = append(compiled, shared.CompilePathPattern(pattern))
	}
	compared, withRawPath := 0, 0
	for _, path := range paths {
		targets := []string{
			(&url.URL{Path: path}).EscapedPath(),
			escapeTarget(path, upperHex, func(int) bool { return true }),
			escapeTarget(path, lowerHex, func(int) bool { return true }),
			escapeTarget(path, upperHex, func(index int) bool { return index%2 == 0 }),
		}
		for _, target := range targets {
			requestURL, err := url.ParseRequestURI(target)
			if err != nil {
				t.Fatalf("failed to parse request target %q: %v", target, err)
			}
			if requestURL.Path != path {
				t.Fatalf("request target %q: expected path %q actual %q", target, path, requestURL.Path)
			}
			if requestURL.RawPath != "" {
				withRawPath++
			}
			req := &http.Request{Method: http.MethodGet, URL: requestURL}
			for i := range predicates {
				expected := compiled[i].Match(path)
				if actual := predicates[i].Test(req); actual != expected {
					t.Errorf("pattern %q request target %q: expected %t actual %t", patterns[i], target, expected, actual)
				}
				compared++
			}
		}
	}

	if withRawPath == 0 {
		t.Errorf("expected request targets with a raw path actual none")
	}
	t.Logf("%d patterns, %d paths, %d comparisons, %d request targets with a raw path",
		len(patterns), len(paths), compared, withRawPath)
}

// TestPath_Test_DoesNotAllocateWithoutEncodedSeparator pins the cost of the common
// request: only a path with an encoded separator may allocate.
func TestPath_Test_DoesNotAllocateWithoutEncodedSeparator(t *testing.T) {
	tests := []struct {
		Name   string
		Target string
	}{
		{
			Name:   "Path should not allocate when nothing is encoded",
			Target: "/api/v1/users/extra/users/7",
		},
		{
			Name:   "Path should not allocate when the encoding leaves no raw path",
			Target: "/api/v1/users/my%20user/users/7",
		},
		{
			Name:   "Path should not allocate when the raw path has no encoded separator",
			Target: "/api/v1/users/%6Dy%20user/users/7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			pred := predicate.NewPathPredicate("/api/*/orders/**", "/api/**/users/*")
			requestURL, err := url.ParseRequestURI(tt.Target)
			if err != nil {
				t.Fatalf("failed to parse request target %q: %v", tt.Target, err)
			}
			req := &http.Request{Method: http.MethodGet, URL: requestURL}

			allocs := testing.AllocsPerRun(100, func() {
				if !pred.Test(req) {
					t.Errorf("expected %t actual %t", true, false)
				}
			})

			if allocs != 0 {
				t.Errorf("expected 0 allocations actual %v", allocs)
			}
		})
	}
}

func TestPathPredicate_Name(t *testing.T) {
	p := predicate.NewPathPredicate("/**")

	if p.Name() != predicate.PathPredicateName {
		t.Errorf("expected %s actual %s", predicate.PathPredicateName, p.Name())
	}
}
