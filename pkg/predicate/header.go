package predicate

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// HeaderPredicateName is the name of the header predicate.
const HeaderPredicateName = "Header"

// ErrInvalidHeaderName is returned when the name of a header predicate is not a header
// field name, so that no request can carry it.
var ErrInvalidHeaderName = errors.New("invalid header name")

// ErrUnavailableHeader is returned when a header predicate is asked for a header that
// net/http removes from the request headers without keeping the value the client sent.
var ErrUnavailableHeader = errors.New("unavailable header")

const (
	hostHeader             = "Host"
	transferEncodingHeader = "Transfer-Encoding"
	trailerHeader          = "Trailer"
)

// headerSource tells where the values of a header are read from.
//
// net/http does not leave every header of a request in Request.Header: a few are moved to
// a field of the request and deleted from the map, where a predicate would look for them
// forever without finding them.
type headerSource uint8

const (
	// headerSourceMap is Request.Header, where all the other headers are.
	headerSourceMap headerSource = iota
	// headerSourceHost is Request.Host. The server deletes the Host header of an HTTP/1
	// request from the map, and HTTP/2 sends the host in the :authority pseudo-header,
	// which never was in it.
	headerSourceHost
	// headerSourceTransferEncoding is Request.TransferEncoding. The server deletes the
	// Transfer-Encoding header from the map whatever its value is.
	headerSourceTransferEncoding
)

// Header is a predicate that checks if a header exists and matches a given regexp.
type Header struct {
	pattern *regexp.Regexp
	name    string
	source  headerSource
}

// NewHeaderPredicate creates a new header predicate.
//
// An empty regexp only requires the header to exist.
//
// The name can be written in any case. Two names are read from the request field net/http
// moves them to, instead of from the request headers:
//
//   - Host is Request.Host: the host as the client sent it, with the port when it sent
//     one. It comes from the Host header, from the :authority pseudo-header of HTTP/2 or
//     from the host of an absolute request target. An empty host counts as a missing
//     header.
//   - Transfer-Encoding is Request.TransferEncoding. The only transfer coding the server
//     accepts is chunked, and that is the value it reports, in lower case.
//
// The name Trailer is rejected with ErrUnavailableHeader, and a name that is not a header
// field name (RFC 9110 section 5.6.2) with ErrInvalidHeaderName: a predicate on any of
// them would be accepted and never match.
func NewHeaderPredicate(header, regexpStr string) (*Header, error) {
	if !isHeaderFieldName(header) {
		return nil, fmt.Errorf(
			"%w %q: it is not a header field name, so no request can carry it", ErrInvalidHeaderName, header)
	}
	// request headers are keyed by their canonical name: resolving it here keeps it out of every evaluation
	name := http.CanonicalHeaderKey(header)
	source := headerSourceMap
	switch name {
	case hostHeader:
		source = headerSourceHost
	case transferEncodingHeader:
		source = headerSourceTransferEncoding
	case trailerHeader:
		// The names a request announces end up as the keys of Request.Trailer, in
		// canonical form and without order, and the header is deleted: the value the
		// client sent, which is what a regexp is written for, cannot be recovered.
		return nil, fmt.Errorf(
			"%w %q: net/http moves it out of the request headers and does not keep the value the client sent",
			ErrUnavailableHeader, header)
	}
	var pattern *regexp.Regexp
	var err error
	if regexpStr != "" {
		pattern, err = regexp.Compile(regexpStr)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp: %w", err)
		}
	}
	return &Header{
		name:    name,
		pattern: pattern,
		source:  source,
	}, nil
}

// NewHeaderPredicateBuilder creates a new header predicate builder.
//
// The 'name' argument is required. The 'regexp' argument is optional: without it the predicate
// only requires the header to exist.
func NewHeaderPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		name, err := shared.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		regex, err := optionalRegexpArg(args)
		if err != nil {
			return nil, err
		}
		return NewHeaderPredicate(name, regex)
	}
}

// Test checks if the header predicate matches the given request.
//
// If the header does not exist, the predicate will return false.
// If the header exists and there is no regexp, the predicate will return true, even when its value is empty.
// If the header exists and there is a regexp, the predicate will return true when at least one of
// its values (which can be empty) matches the regexp.
//
// A value matches the regexp as a whole, the way it was received, or through any element
// of it read as a comma-separated list (RFC 9110 section 5.6.1): "beta, v3" in one line is
// the same header as "beta" and "v3" in two, and an intermediary may turn one into the
// other. The elements are trimmed of the whitespace around them, the empty ones are
// skipped and a comma inside a quoted string does not separate anything.
func (p *Header) Test(request *http.Request) bool {
	var values []string
	switch p.source {
	case headerSourceMap:
		values = request.Header[p.name]
	case headerSourceTransferEncoding:
		values = request.TransferEncoding
	case headerSourceHost:
		return p.testHost(request.Host)
	}
	if p.pattern == nil {
		return len(values) > 0
	}
	for _, value := range values {
		if p.pattern.MatchString(value) {
			return true
		}
		// A value without a comma is its own and only element, which has just been tested:
		// looking for the comma is all the list costs to the values that are not one.
		if strings.IndexByte(value, ',') != -1 && p.matchesListElement(value) {
			return true
		}
	}
	return false
}

// Name returns the name of the predicate.
func (p *Header) Name() string {
	return HeaderPredicateName
}

// testHost tests the host of the request, which is a single value and not a list.
func (p *Header) testHost(host string) bool {
	if host == "" {
		// net/http has no way to tell a missing Host header from an empty one.
		return false
	}
	return p.pattern == nil || p.pattern.MatchString(host)
}

// matchesListElement reports whether the regexp matches any element of the value read as
// a comma-separated list.
//
// It walks the value in place: nothing is allocated.
func (p *Header) matchesListElement(value string) bool {
	for value != "" {
		end := listElementEnd(value)
		if element := trimOptionalWhitespace(value[:end]); element != "" && p.pattern.MatchString(element) {
			return true
		}
		if end == len(value) {
			break
		}
		value = value[end+1:]
	}
	return false
}

// listElementEnd returns the offset of the comma that ends the first element of the list,
// or the length of the list when it has a single element. A comma inside a quoted string
// (RFC 9110 section 5.6.4) is part of the element. So is one after a quote that is never
// closed, which leaves the rest of the list as one element.
func listElementEnd(list string) int {
	quoted := false
	for pos := 0; pos < len(list); pos++ {
		switch list[pos] {
		case '"':
			quoted = !quoted
		case '\\':
			if quoted {
				// A quoted pair: the next character is taken as it is, even a quote.
				pos++
			}
		case ',':
			if !quoted {
				return pos
			}
		}
	}
	return len(list)
}

// trimOptionalWhitespace removes the optional whitespace (RFC 9110 section 5.6.3: spaces
// and horizontal tabs) around a list element.
func trimOptionalWhitespace(element string) string {
	start, end := 0, len(element)
	for start < end && isOptionalWhitespace(element[start]) {
		start++
	}
	for end > start && isOptionalWhitespace(element[end-1]) {
		end--
	}
	return element[start:end]
}

func isOptionalWhitespace(char byte) bool {
	return char == ' ' || char == '\t'
}

// isHeaderFieldName reports whether the name is a token (RFC 9110 section 5.6.2), which
// is what a header field name is. The server answers 400 Bad Request to anything else, so
// such a header never reaches the predicate. That includes the pseudo-headers of HTTP/2
// (":authority", ":path" and the rest), which are not headers and are not in
// Request.Header.
func isHeaderFieldName(name string) bool {
	if name == "" {
		return false
	}
	for i := range len(name) {
		if !isTokenChar(name[i]) {
			return false
		}
	}
	return true
}

func isTokenChar(char byte) bool {
	switch {
	case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
		return true
	default:
		return strings.IndexByte("!#$%&'*+-.^_`|~", char) != -1
	}
}
