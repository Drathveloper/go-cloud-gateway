package common

import (
	"regexp"
	"strings"
)

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

func simpleMatch(pattern, path string) bool {
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			return true
		case '?':
			if len(path) == 0 {
				return false
			}
		default:
			if i >= len(path) || pattern[i] != path[i] {
				return false
			}
		}
	}
	return len(pattern) == len(path)
}

func HostMatcher(pattern, host string) bool {
	if pattern == "**" {
		return true
	}
	regexPattern := convertPatternToRegex(pattern)
	matched, err := regexp.MatchString(regexPattern, host)
	if err != nil {
		return false
	}
	return matched
}

func convertPatternToRegex(pattern string) string {
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
