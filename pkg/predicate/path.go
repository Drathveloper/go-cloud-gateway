package predicate

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// PathPredicateName is the name of the path predicate.
const PathPredicateName = "Path"

// encodedSeparatorLen is the length of an encoded path separator: "%2F".
const encodedSeparatorLen = 3

// Path is a predicate that checks if the request path matches a given pattern.
type Path struct {
	patterns []shared.PathPattern
}

// NewPathPredicate creates a new path predicate.
//
// The patterns are compiled here, once, instead of on every request.
func NewPathPredicate(patterns ...string) *Path {
	compiled := make([]shared.PathPattern, 0, len(patterns))
	for _, pattern := range patterns {
		compiled = append(compiled, shared.CompilePathPattern(pattern))
	}
	return &Path{
		patterns: compiled,
	}
}

// NewPathPredicateBuilder creates a new path predicate builder.
func NewPathPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		patterns, err := shared.ConvertToStringSlice(args["patterns"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'patterns' attribute: %w", err)
		}
		return NewPathPredicate(patterns...), nil
	}
}

// Test checks if the request path matches the given patterns.
//
// If the request path does not match any pattern, the predicate will return false.
// If the request path matches at least one pattern, the predicate will return true.
//
// The path is matched segment by segment as the client sent it: a segment ends at a '/'
// of the request target and not at an encoded one ("%2F"), which is one more character
// of its segment. Every other escape compares equal to the character it stands for:
// "/files/my%20file" matches the pattern "/files/my file".
func (p *Path) Test(request *http.Request) bool {
	// net/url only sets RawPath when the client sent a path that is not the default
	// encoding of Path, and a path with an encoded separator never is that encoding. An
	// empty RawPath thus rules it out: the common request pays for this comparison alone.
	if request.URL.RawPath != "" {
		if escapedPath, ok := pathWithEncodedSeparator(request.URL); ok {
			return p.testEscaped(escapedPath)
		}
	}
	for i := range p.patterns {
		if p.patterns[i].Match(request.URL.Path) {
			return true
		}
	}
	return false
}

// Name returns the name of the predicate.
func (p *Path) Name() string {
	return PathPredicateName
}

func (p *Path) testEscaped(escapedPath string) bool {
	for i := range p.patterns {
		if p.patterns[i].MatchEscaped(escapedPath) {
			return true
		}
	}
	return false
}

// pathWithEncodedSeparator returns the escaped path of the url when it has an encoded
// separator ("%2F"), which is the only case where the segments of the escaped path are
// not the ones of url.Path.
//
// A RawPath without one is not looked at any further: its segments are the ones of Path.
// A RawPath that is not a valid encoding of Path is ignored by EscapedPath, which then
// encodes Path, where every separator is a real one. The escaped path is the one the
// backend receives too: the route is chosen on the same segments it is going to see.
//
// EscapedPath decodes RawPath to compare it with Path, which allocates: that is the price
// of a request with an encoded separator, and of no other.
func pathWithEncodedSeparator(requestURL *url.URL) (string, bool) {
	if !hasEncodedSeparator(requestURL.RawPath) {
		return "", false
	}
	escapedPath := requestURL.EscapedPath()
	return escapedPath, hasEncodedSeparator(escapedPath)
}

// hasEncodedSeparator reports whether the escaped path has an encoded '/'.
func hasEncodedSeparator(escapedPath string) bool {
	rest := escapedPath
	for {
		escape := strings.IndexByte(rest, '%')
		if escape == -1 || escape+encodedSeparatorLen > len(rest) {
			return false
		}
		if rest[escape+1] == '2' && (rest[escape+2] == 'F' || rest[escape+2] == 'f') {
			return true
		}
		rest = rest[escape+1:]
	}
}
