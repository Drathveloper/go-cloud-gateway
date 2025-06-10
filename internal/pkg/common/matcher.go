package common

import (
	"regexp"
	"strings"
)

// PathMatcher matches the path pattern with the path.
//
// The path pattern is a string that contains the path segments separated by '/'.
//
// The path is a string that contains the path segments separated by '/'.
//
// The path pattern can contain the following special characters:
//
// 1. '*' matches any number of characters.
// 2. '**' matches any number of characters including the '/' character.
// 3. '?' matches a single character.
//
//nolint:cyclop
func PathMatcher(pattern, path string) bool {
	if pattern == "" {
		return path == ""
	}
	if pattern == "/**" {
		return true
	}
	patternSegments := strings.Split(strings.Trim(pattern, "/"), "/")
	pathSegments := strings.Split(strings.Trim(path, "/"), "/")
	patternIdx, pathIdx := 0, 0
	for patternIdx < len(patternSegments) && pathIdx < len(pathSegments) {
		currentPattern := patternSegments[patternIdx]
		currentPath := pathSegments[pathIdx]
		if currentPattern == "**" {
			if patternIdx+1 < len(patternSegments) {
				nextPattern := patternSegments[patternIdx+1]
				for pathIdx < len(pathSegments) && pathSegments[pathIdx] != nextPattern {
					pathIdx++
				}
				if pathIdx == len(pathSegments) {
					return false
				}
				patternIdx += 2
				pathIdx++
			} else {
				return true
			}
			continue
		}
		if !simpleMatch(currentPattern, currentPath) {
			return false
		}
		patternIdx++
		pathIdx++
	}
	if patternIdx < len(patternSegments) && patternSegments[patternIdx] == "**" {
		return true
	}
	return patternIdx == len(patternSegments) && pathIdx == len(pathSegments)
}

//nolint:cyclop
func simpleMatch(pattern, str string) bool {
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

// HostMatcher matches the host pattern with the host.
//
// The host pattern is a string that contains the host segments separated by '.'.
//
// The host is a string that contains the host segments separated by '.'.
//
// The host pattern can contain the following special characters:
//
// 1. '*' matches any number of characters.
// 2. '**' matches any number of characters including the '/' character.
// 3. '?' matches a single character.
func HostMatcher(pattern *regexp.Regexp, host string) bool {
	if pattern == nil {
		return true
	}
	if pattern.MatchString(host) {
		return true
	}
	return false
}

// ConvertPatternToRegex converts the given pattern to a regular expression.
func ConvertPatternToRegex(pattern string) string {
	regex := strings.ReplaceAll(pattern, ".", "\\.")
	regex = strings.ReplaceAll(regex, "*", "[^.]+")
	regex = strings.ReplaceAll(regex, "[^.]+[^.]+", ".+")
	if !strings.HasPrefix(regex, "^") {
		regex = "^" + regex
	}
	if !strings.HasSuffix(regex, "$") {
		regex += "$"
	}
	return regex
}
