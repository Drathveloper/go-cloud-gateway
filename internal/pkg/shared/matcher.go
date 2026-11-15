package shared

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

const (
	pathSeparator     = '/'
	recursiveWildcard = "**"
)

// PathPattern is a path pattern compiled once, so that matching a request path does not
// have to split the pattern again.
//
// A pattern is a list of segments separated by '/'. Leading and trailing separators are
// not significant. A segment is either:
//
//   - '**', which matches any number of path segments, including none.
//   - a literal that may contain '*', which matches any number of characters within the
//     segment, and '?', which matches exactly one character.
//
// The empty pattern only matches the empty path.
//
// Match takes a decoded path and MatchEscaped an escaped one. They only disagree on a path
// with an encoded separator ("%2F"): decoded, it splits the segment it is in.
type PathPattern struct {
	segments []string
	empty    bool
	matchAll bool
}

// CompilePathPattern compiles the given path pattern.
func CompilePathPattern(pattern string) PathPattern {
	if pattern == "" {
		return PathPattern{empty: true}
	}
	segments := strings.Split(strings.Trim(pattern, string(pathSeparator)), string(pathSeparator))
	return PathPattern{
		segments: segments,
		matchAll: len(segments) == 1 && segments[0] == recursiveWildcard,
	}
}

// Match reports whether the decoded path matches the pattern.
func (p *PathPattern) Match(path string) bool {
	if p.empty {
		return path == ""
	}
	if p.matchAll {
		return true
	}
	return p.matchSegments(strings.Trim(path, string(pathSeparator)))
}

// MatchEscaped reports whether the escaped path, the one the client sent, matches the
// pattern.
//
// The path is split into segments on the separators it has before decoding, and each
// segment is percent-decoded while it is compared. An encoded separator ("%2F") is thus
// one more character of its segment, which '*' and '?' can match, instead of the end of
// it: "/static/a%2Fb.css" has the segments "static" and "a/b.css". Any other escape
// compares equal to the character it stands for, as in Match: for a path without an
// encoded separator both report the same.
//
// The path is expected to be a valid encoding, such as the result of url.URL.EscapedPath.
// A '%' that does not start an escape is taken as it is. Like Match, it walks the path in
// place without allocating.
func (p *PathPattern) MatchEscaped(escapedPath string) bool {
	if p.empty {
		return escapedPath == ""
	}
	if p.matchAll {
		return true
	}
	return p.matchEscapedSegments(strings.Trim(escapedPath, string(pathSeparator)))
}

// matchSegments matches the pattern against a path without leading and trailing
// separators.
//
// A '**' first matches no segment at all and is extended one segment at a time whenever
// the rest of the pattern fails to match. Only the last '**' has to be revisited: an
// earlier one could not find a match that the last one, which can skip any number of
// segments, would miss.
func (p *PathPattern) matchSegments(path string) bool {
	patternIdx, pathPos := 0, 0
	wildcardIdx, wildcardPos := -1, 0
	// pathPos is the offset of the current segment. The path is exhausted once it moves
	// past the end: an empty path still has one (empty) segment.
	for pathPos <= len(path) {
		if patternIdx < len(p.segments) && p.segments[patternIdx] == recursiveWildcard {
			wildcardIdx, wildcardPos = patternIdx, pathPos
			patternIdx++
			continue
		}
		segment, next := nextSegment(path, pathPos)
		if patternIdx < len(p.segments) && simpleMatch(p.segments[patternIdx], segment) {
			patternIdx++
			pathPos = next
			continue
		}
		if wildcardIdx == -1 {
			return false
		}
		_, wildcardPos = nextSegment(path, wildcardPos)
		patternIdx, pathPos = wildcardIdx+1, wildcardPos
	}
	for patternIdx < len(p.segments) && p.segments[patternIdx] == recursiveWildcard {
		patternIdx++
	}
	return patternIdx == len(p.segments)
}

// matchEscapedSegments is matchSegments for an escaped path: the segments are the ones of
// the escaped path and are decoded by simpleMatchEscaped while they are compared.
func (p *PathPattern) matchEscapedSegments(path string) bool {
	patternIdx, pathPos := 0, 0
	wildcardIdx, wildcardPos := -1, 0
	for pathPos <= len(path) {
		if patternIdx < len(p.segments) && p.segments[patternIdx] == recursiveWildcard {
			wildcardIdx, wildcardPos = patternIdx, pathPos
			patternIdx++
			continue
		}
		segment, next := nextSegment(path, pathPos)
		if patternIdx < len(p.segments) && simpleMatchEscaped(p.segments[patternIdx], segment) {
			patternIdx++
			pathPos = next
			continue
		}
		if wildcardIdx == -1 {
			return false
		}
		_, wildcardPos = nextSegment(path, wildcardPos)
		patternIdx, pathPos = wildcardIdx+1, wildcardPos
	}
	for patternIdx < len(p.segments) && p.segments[patternIdx] == recursiveWildcard {
		patternIdx++
	}
	return patternIdx == len(p.segments)
}

// nextSegment returns the path segment that starts at pos and the offset of the segment
// after it, which is past the end of the path for the last segment.
func nextSegment(path string, pos int) (string, int) {
	end := strings.IndexByte(path[pos:], pathSeparator)
	if end == -1 {
		return path[pos:], len(path) + 1
	}
	return path[pos : pos+end], pos + end + 1
}

//nolint:cyclop
func simpleMatch(pattern, str string) bool {
	pIdx, sIdx := 0, 0
	starIdx, match := -1, 0
	for sIdx < len(str) {
		switch {
		// Before the literal comparison: a '*' of the pattern is a wildcard also when
		// the segment has a '*' of its own at that place.
		case pIdx < len(pattern) && pattern[pIdx] == '*':
			starIdx = pIdx
			match = sIdx
			pIdx++
		case pIdx < len(pattern) && (pattern[pIdx] == str[sIdx] || pattern[pIdx] == '?'):
			pIdx++
			sIdx++
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

// simpleMatchEscaped is simpleMatch for an escaped segment: every step reads one decoded
// character of the segment, which takes one byte of it or the three of an escape.
//
//nolint:cyclop
func simpleMatchEscaped(pattern, str string) bool {
	pIdx, sIdx := 0, 0
	starIdx, match := -1, 0
	for sIdx < len(str) {
		char, width := escapedChar(str, sIdx)
		switch {
		case pIdx < len(pattern) && pattern[pIdx] == '*':
			starIdx = pIdx
			match = sIdx
			pIdx++
		case pIdx < len(pattern) && (pattern[pIdx] == char || pattern[pIdx] == '?'):
			pIdx++
			sIdx += width
		case starIdx != -1:
			pIdx = starIdx + 1
			_, skipped := escapedChar(str, match)
			match += skipped
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

// escapedChar returns the decoded character at pos of an escaped string and the number of
// bytes it takes there: three for an escape ("%2F") and one for anything else.
func escapedChar(str string, pos int) (byte, int) {
	const escapeLen = 3
	if str[pos] != '%' || pos+escapeLen > len(str) {
		return str[pos], 1
	}
	high, low := hexValue(str[pos+1]), hexValue(str[pos+2])
	if high == notHex || low == notHex {
		return str[pos], 1
	}
	return high<<4 | low, escapeLen
}

// notHex is the value hexValue returns for a character that is not a hex digit.
const notHex = 0xff

// hexValue returns the value of a hex digit, or notHex.
func hexValue(char byte) byte {
	const letterOffset = 10
	switch {
	case char >= '0' && char <= '9':
		return char - '0'
	case char >= 'a' && char <= 'f':
		return char - 'a' + letterOffset
	case char >= 'A' && char <= 'F':
		return char - 'A' + letterOffset
	default:
		return notHex
	}
}

// HostPattern is a host pattern compiled once.
//
// A pattern is a host name whose labels are separated by '.'. It can contain:
//
//   - '*', which matches one or more characters within a label.
//   - '**', which matches one or more characters across labels.
//   - '?', which matches exactly one character within a label.
//
// Everything else is literal. The pattern "**" matches every host.
//
// Matching is case-insensitive, as host names are. The port of the host is ignored unless
// the pattern has one itself: "example.org" matches "example.org:8080", while
// "example.org:8080" only matches that host on that port.
type HostPattern struct {
	regex    *regexp.Regexp
	literal  string
	matchAll bool
	withPort bool
}

// CompileHostPattern compiles the given host pattern.
func CompileHostPattern(pattern string) (HostPattern, error) {
	pattern = strings.ToLower(pattern)
	if pattern == recursiveWildcard {
		return HostPattern{matchAll: true}, nil
	}
	_, port, err := net.SplitHostPort(pattern)
	compiled := HostPattern{withPort: err == nil && port != ""}
	if !strings.ContainsAny(pattern, "*?") {
		// A pattern without wildcards is compared as a string: no regexp is needed.
		compiled.literal = pattern
		return compiled, nil
	}
	compiled.regex, err = regexp.Compile(hostPatternToRegex(pattern))
	if err != nil {
		return HostPattern{}, fmt.Errorf("failed to compile host pattern: %w", err)
	}
	return compiled, nil
}

// Match reports whether the host, as sent in the Host header, matches the pattern.
func (p *HostPattern) Match(host string) bool {
	if p.matchAll {
		return true
	}
	if !p.withPort {
		host = hostWithoutPort(host)
	}
	host = strings.ToLower(host)
	if p.regex == nil {
		return host == p.literal
	}
	return p.regex.MatchString(host)
}

// hostPatternToRegex converts a host pattern into an anchored regular expression. The
// literal parts are quoted, so regular expression syntax in a pattern has no meaning.
func hostPatternToRegex(pattern string) string {
	var regex strings.Builder
	regex.WriteByte('^')
	for pattern != "" {
		switch {
		case strings.HasPrefix(pattern, recursiveWildcard):
			regex.WriteString(".+")
			pattern = strings.TrimLeft(pattern, "*")
		case pattern[0] == '*':
			regex.WriteString("[^.]+")
			pattern = pattern[1:]
		case pattern[0] == '?':
			regex.WriteString("[^.]")
			pattern = pattern[1:]
		default:
			end := strings.IndexAny(pattern, "*?")
			if end == -1 {
				end = len(pattern)
			}
			regex.WriteString(regexp.QuoteMeta(pattern[:end]))
			pattern = pattern[end:]
		}
	}
	regex.WriteByte('$')
	return regex.String()
}

// hostWithoutPort returns the host without its port, if it has one. An IPv6 literal
// keeps its brackets, so it reads the same with and without a port. It does not allocate,
// unlike net.SplitHostPort on a host without a port, which is the common case.
func hostWithoutPort(host string) string {
	colon := strings.LastIndexByte(host, ':')
	if colon == -1 {
		return host
	}
	// The last colon belongs to an IPv6 literal: "[::1]" or an unbracketed "::1".
	if strings.IndexByte(host[colon:], ']') != -1 {
		return host
	}
	if host[0] != '[' && strings.IndexByte(host[:colon], ':') != -1 {
		return host
	}
	return host[:colon]
}
