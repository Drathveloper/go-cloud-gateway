package shared_test

import (
	"math/rand"
	"net/url"
	"strings"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

// The functions prefixed with reference are the path matcher of v1.7.0, copied as it was
// released. It matched the decoded path and knew nothing about escapes: it is what
// MatchEscaped has to agree with for every path without an encoded separator.

func referenceMatch(pattern, path string) bool {
	if pattern == "" {
		return path == ""
	}
	segments := strings.Split(strings.Trim(pattern, "/"), "/")
	if len(segments) == 1 && segments[0] == "**" {
		return true
	}
	return referenceMatchSegments(segments, strings.Trim(path, "/"))
}

func referenceMatchSegments(segments []string, path string) bool {
	patternIdx, pathPos := 0, 0
	wildcardIdx, wildcardPos := -1, 0
	for pathPos <= len(path) {
		if patternIdx < len(segments) && segments[patternIdx] == "**" {
			wildcardIdx, wildcardPos = patternIdx, pathPos
			patternIdx++
			continue
		}
		segment, next := referenceNextSegment(path, pathPos)
		if patternIdx < len(segments) && referenceSimpleMatch(segments[patternIdx], segment) {
			patternIdx++
			pathPos = next
			continue
		}
		if wildcardIdx == -1 {
			return false
		}
		_, wildcardPos = referenceNextSegment(path, wildcardPos)
		patternIdx, pathPos = wildcardIdx+1, wildcardPos
	}
	for patternIdx < len(segments) && segments[patternIdx] == "**" {
		patternIdx++
	}
	return patternIdx == len(segments)
}

func referenceNextSegment(path string, pos int) (string, int) {
	end := strings.IndexByte(path[pos:], '/')
	if end == -1 {
		return path[pos:], len(path) + 1
	}
	return path[pos : pos+end], pos + end + 1
}

func referenceSimpleMatch(pattern, str string) bool {
	pIdx, sIdx := 0, 0
	starIdx, match := -1, 0
	for sIdx < len(str) {
		switch {
		case pIdx < len(pattern) && (pattern[pIdx] == str[sIdx] || pattern[pIdx] == '?'):
			pIdx++
			sIdx++
		case pIdx < len(pattern) && pattern[pIdx] == '*':
			starIdx = pIdx
			match = sIdx
			pIdx++
		case starIdx != -1:
			pIdx = starIdx + 1
			match++
			sIdx = match
		default:
			return false
		}
	}
	for pIdx < len(pattern) && pattern[pIdx] == '*' {
		pIdx++
	}

	return pIdx == len(pattern)
}

const upperHex, lowerHex = "0123456789ABCDEF", "0123456789abcdef"

// escapePath percent-encodes the bytes of the decoded path that shouldEscape picks. A
// separator is never encoded: it would no longer be one. A percent sign always is: left
// alone, it would start an escape.
func escapePath(path, hex string, shouldEscape func(index int) bool) string {
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

// escapedForms returns encodings of the decoded path that leave the separators as they
// are: the one net/url produces, nothing encoded but what has to be, everything encoded
// in upper and in lower case hex, and every other byte encoded.
func escapedForms(path string) []string {
	defaultForm := (&url.URL{Path: path}).EscapedPath()
	return []string{
		defaultForm,
		escapePath(path, upperHex, func(int) bool { return false }),
		escapePath(path, upperHex, func(int) bool { return true }),
		escapePath(path, lowerHex, func(int) bool { return true }),
		escapePath(path, upperHex, func(index int) bool { return index%2 == 0 }),
		escapePath(path, lowerHex, func(index int) bool { return index%2 == 1 }),
	}
}

// joinSegments returns every way of joining up to maxSegments of the given segments.
func joinSegments(segments []string, maxSegments int) []string {
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

// assertSameAsReference checks, for a decoded path, that Match still reports what the
// matcher of v1.7.0 did and that MatchEscaped reports it too for every encoding of the
// path. It returns the number of comparisons made.
func assertSameAsReference(t *testing.T, patterns, paths []string) int {
	t.Helper()
	compiled := make([]shared.PathPattern, 0, len(patterns))
	for _, pattern := range patterns {
		compiled = append(compiled, shared.CompilePathPattern(pattern))
	}
	compared := 0
	for _, path := range paths {
		forms := escapedForms(path)
		for i, pattern := range patterns {
			expected := referenceMatch(pattern, path)
			if actual := compiled[i].Match(path); actual != expected {
				t.Errorf("pattern %q path %q: expected %t actual %t", pattern, path, expected, actual)
			}
			compared++
			for _, form := range forms {
				if actual := compiled[i].MatchEscaped(form); actual != expected {
					t.Errorf("pattern %q path %q escaped as %q: expected %t actual %t",
						pattern, path, form, expected, actual)
				}
				compared++
			}
		}
	}
	return compared
}

// TestPathPattern_MatchEscaped_SameAsDecodedMatcher is the proof that matching the
// escaped path changes nothing but the encoded separator: over every combination of a
// set of patterns and paths, the matcher of v1.7.0 on the decoded path and MatchEscaped
// on any encoding of it that keeps the separators report the same.
func TestPathPattern_MatchEscaped_SameAsDecodedMatcher(t *testing.T) {
	patternSegments := []string{"a", "b", "a b", "*", "?", "**", "a*", "*b", "?*b", "*.c"}
	pathSegments := []string{"a", "b", "ab", "", "a b", "a.c", "a*b", "a%b?"}

	patterns := joinSegments(patternSegments, 3)
	// Leading and trailing separators are not significant in a pattern, and the empty
	// pattern is a case of its own.
	patterns = append(patterns, "", "/", "//", "a/b", "a/*/", "/a/**/", "**", "**/", "/**/**", "/a//b", "//a")
	// Four segments are only built out of the first five, to keep the test fast.
	paths := joinSegments(pathSegments, 3)
	paths = append(paths, joinSegments(pathSegments[:5], 4)...)
	paths = append(paths, "", "/", "//", "a", "a/b", "/a/", "/a/b//")

	compared := assertSameAsReference(t, patterns, paths)

	t.Logf("%d patterns, %d paths, %d comparisons", len(patterns), len(paths), compared)
}

// randomString returns a string of up to maxLen characters of the alphabet.
func randomString(random *rand.Rand, alphabet []string, maxLen int) string {
	var builder strings.Builder
	for range random.Intn(maxLen + 1) {
		builder.WriteString(alphabet[random.Intn(len(alphabet))])
	}
	return builder.String()
}

// TestPathPattern_MatchEscaped_SameAsDecodedMatcherOnRandomInput does the same over
// patterns and paths that follow no structure: longer, with wildcards and separators
// anywhere and with bytes that only an escape can put in a path.
func TestPathPattern_MatchEscaped_SameAsDecodedMatcherOnRandomInput(t *testing.T) {
	const rounds, patternsPerRound, pathsPerRound = 2000, 12, 12
	// The separator and the wildcards are repeated so that they show up often enough.
	patternAlphabet := []string{"/", "/", "/", "*", "*", "**", "?", "a", "b", "c", ".", " ", "%", "é"}
	pathAlphabet := []string{"/", "/", "/", "a", "a", "b", "b", "c", ".", " ", "%", "*", "?", "é", "\x00", "\xff", "#", "+", "2", "F"}
	// The seed is fixed: a failure has to be reproducible.
	random := rand.New(rand.NewSource(20261115)) //nolint:gosec // test data, not a secret

	compared := 0
	for range rounds {
		patterns := make([]string, 0, patternsPerRound)
		for range patternsPerRound {
			patterns = append(patterns, randomString(random, patternAlphabet, 10))
		}
		paths := make([]string, 0, pathsPerRound)
		for range pathsPerRound {
			paths = append(paths, randomString(random, pathAlphabet, 14))
		}
		compared += assertSameAsReference(t, patterns, paths)
	}

	t.Logf("%d comparisons", compared)
}
