package predicate_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

func TestNewHeaderPredicateBuilder(t *testing.T) {
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
			name: "build should succeed when header regexp argument is not present",
			args: map[string]any{
				"name": "First",
			},
			expectedErr: nil,
		},
		{
			name: "build should succeed when header regexp argument is null",
			args: map[string]any{
				"name":   "First",
				"regexp": nil,
			},
			expectedErr: nil,
		},
		{
			name: "build should fail when header name argument is not valid",
			args: map[string]any{
				"regexp": "any1",
			},
			expectedErr: errors.New("failed to convert 'name' attribute: value is required"),
		},
		{
			name: "build should fail when header regexp argument is not a string",
			args: map[string]any{
				"name":   "First",
				"regexp": 1234,
			},
			expectedErr: errors.New("failed to convert 'regexp' attribute: value is required to be a valid string"),
		},
		{
			name: "build should fail when header regexp argument is not a valid regexp",
			args: map[string]any{
				"name":   "First",
				"regexp": "[",
			},
			expectedErr: errors.New("invalid regexp: error parsing regexp: missing closing ]: `[`"),
		},
		{
			name: "build should succeed when header name is host",
			args: map[string]any{
				"name":   "host",
				"regexp": `^example\.org$`,
			},
			expectedErr: nil,
		},
		{
			name: "build should succeed when header name is transfer encoding",
			args: map[string]any{
				"name": "Transfer-Encoding",
			},
			expectedErr: nil,
		},
		{
			name: "build should fail when header name is trailer",
			args: map[string]any{
				"name": "Trailer",
			},
			expectedErr: errors.New(`unavailable header "Trailer": net/http moves it out of the request headers ` +
				"and does not keep the value the client sent"),
		},
		{
			name: "build should fail when header name is trailer in another case",
			args: map[string]any{
				"name":   "tRAILER",
				"regexp": "any1",
			},
			expectedErr: errors.New(`unavailable header "tRAILER": net/http moves it out of the request headers ` +
				"and does not keep the value the client sent"),
		},
		{
			name: "build should fail when header name is empty",
			args: map[string]any{
				"name": "",
			},
			expectedErr: errors.New(`invalid header name "": it is not a header field name, so no request can carry it`),
		},
		{
			name: "build should fail when header name has a character a header field name cannot have",
			args: map[string]any{
				"name": "X Test Id",
			},
			expectedErr: errors.New(
				`invalid header name "X Test Id": it is not a header field name, so no request can carry it`),
		},
		{
			name: "build should fail when header name is a pseudo-header",
			args: map[string]any{
				"name": ":authority",
			},
			expectedErr: errors.New(
				`invalid header name ":authority": it is not a header field name, so no request can carry it`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := predicate.NewHeaderPredicateBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestNewHeaderPredicateBuilder_WithoutRegexp(t *testing.T) {
	tests := []struct {
		header   http.Header
		name     string
		expected bool
	}{
		{
			name:     "built predicate should match when header is present",
			header:   http.Header{"X-Test-Id": {"666"}},
			expected: true,
		},
		{
			name:     "built predicate shouldn't match when header is not present",
			header:   http.Header{"X-Test-Other": {"666"}},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := predicate.NewHeaderPredicateBuilder().Build(map[string]any{"name": "X-Test-Id"})
			if err != nil {
				t.Fatalf("expected err %v actual %v", nil, err)
			}
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test", nil)
			req.Header = tt.header

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}

func TestHeaderPredicate_Test(t *testing.T) {
	tests := []struct {
		header      http.Header
		name        string
		headerName  string
		headerValue string
		expected    bool
	}{
		{
			name:        "test should match when header is present and no value",
			headerName:  "X-Test-Id",
			headerValue: "",
			header:      http.Header{"X-Test-Id": {"666"}},
			expected:    true,
		},
		{
			name:        "test shouldn't match when header is not present and no regex",
			headerName:  "X-Test-Id",
			headerValue: "",
			header:      http.Header{"X-Test-Other": {"666"}},
			expected:    false,
		},
		{
			name:        "test should match when header is present and value matches",
			headerName:  "X-Test-Id",
			headerValue: "1234",
			header:      http.Header{"X-Test-Id": {"1234"}},
			expected:    true,
		},
		{
			name:        "test shouldn't match when header is not present and regex",
			headerName:  "X-Test-Id",
			headerValue: "1234",
			header:      http.Header{"X-Test-Other": {"5678"}},
			expected:    false,
		},
		{
			name:        "test shouldn't match when header is present and value doesn't match",
			headerName:  "X-Test-Id",
			headerValue: "1234",
			header:      http.Header{"X-Test-Id": {"5678"}},
			expected:    false,
		},
		{
			name:        "test should match when header is present with empty value and no regex",
			headerName:  "X-Test-Id",
			headerValue: "",
			header:      http.Header{"X-Test-Id": {""}},
			expected:    true,
		},
		{
			name:        "test should match when header is present with empty value and regex matches empty value",
			headerName:  "X-Test-Id",
			headerValue: "^$",
			header:      http.Header{"X-Test-Id": {""}},
			expected:    true,
		},
		{
			name:        "test shouldn't match when header is present with empty value and regex doesn't match empty value",
			headerName:  "X-Test-Id",
			headerValue: "1234",
			header:      http.Header{"X-Test-Id": {""}},
			expected:    false,
		},
		{
			name:        "test shouldn't match when header has no values and no regex",
			headerName:  "X-Test-Id",
			headerValue: "",
			header:      http.Header{"X-Test-Id": {}},
			expected:    false,
		},
		{
			name:        "test should match when header has several values and no regex",
			headerName:  "X-Test-Id",
			headerValue: "",
			header:      http.Header{"X-Test-Id": {"5678", "1234"}},
			expected:    true,
		},
		{
			name:        "test should match when header has several values and only the last one matches",
			headerName:  "X-Test-Id",
			headerValue: "1234",
			header:      http.Header{"X-Test-Id": {"5678", "", "1234"}},
			expected:    true,
		},
		{
			name:        "test shouldn't match when header has several values and none matches",
			headerName:  "X-Test-Id",
			headerValue: "1234",
			header:      http.Header{"X-Test-Id": {"5678", "9012"}},
			expected:    false,
		},
		{
			name:        "test should match when header name is not configured in canonical form",
			headerName:  "x-test-ID",
			headerValue: "1234",
			header:      http.Header{"X-Test-Id": {"1234"}},
			expected:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := predicate.NewHeaderPredicate(tt.headerName, tt.headerValue)
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test", nil)
			req.Header = tt.header

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}

func TestNewHeaderPredicate_Errors(t *testing.T) {
	tests := []struct {
		expectedErr error
		name        string
		headerName  string
	}{
		{
			name:        "new should fail with unavailable header when header name is trailer",
			headerName:  "trailer",
			expectedErr: predicate.ErrUnavailableHeader,
		},
		{
			name:        "new should fail with invalid header name when header name is not a token",
			headerName:  "X-Test-Id:",
			expectedErr: predicate.ErrInvalidHeaderName,
		},
		{
			name:        "new should fail with invalid header name when header name is not ascii",
			headerName:  "X-Tést-Id",
			expectedErr: predicate.ErrInvalidHeaderName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := predicate.NewHeaderPredicate(tt.headerName, "")

			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected err %v actual %v", tt.expectedErr, err)
			}
			if actual != nil {
				t.Errorf("expected nil predicate actual %v", actual)
			}
		})
	}
}

func TestNewHeaderPredicate_AcceptsEveryTokenCharacter(t *testing.T) {
	// RFC 9110 section 5.6.2: every character a header field name can have.
	name := "!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	p, err := predicate.NewHeaderPredicate(name, "")
	if err != nil {
		t.Fatalf("expected nil err actual %v", err)
	}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test", nil)
	req.Header.Set(name, "1234")

	if !p.Test(req) {
		t.Errorf("expected %t actual %t", true, false)
	}
}

// net/http moves the Host header to Request.Host and deletes it from the headers, so the
// requests of these tests carry the host there and nowhere else, like the ones a server
// hands to its handler.
func TestHeaderPredicate_Test_Host(t *testing.T) {
	tests := []struct {
		name       string
		headerName string
		regexp     string
		host       string
		expected   bool
	}{
		{
			name:       "test should match when host is present and no regex",
			headerName: "Host",
			regexp:     "",
			host:       "example.org",
			expected:   true,
		},
		{
			name:       "test shouldn't match when host is empty and no regex",
			headerName: "Host",
			regexp:     "",
			host:       "",
			expected:   false,
		},
		{
			name:       "test should match when host matches the regex",
			headerName: "Host",
			regexp:     `^(api|admin)\.example\.org$`,
			host:       "admin.example.org",
			expected:   true,
		},
		{
			name:       "test shouldn't match when host doesn't match the regex",
			headerName: "Host",
			regexp:     `^(api|admin)\.example\.org$`,
			host:       "www.example.org",
			expected:   false,
		},
		{
			name:       "test should match the host with the port the client sent",
			headerName: "Host",
			regexp:     `^example\.org:8443$`,
			host:       "example.org:8443",
			expected:   true,
		},
		{
			name:       "test shouldn't match a regex without port when the client sent one",
			headerName: "Host",
			regexp:     `^example\.org$`,
			host:       "example.org:8443",
			expected:   false,
		},
		{
			name:       "test should match the host as received, without changing its case",
			headerName: "Host",
			regexp:     `^Example\.ORG$`,
			host:       "Example.ORG",
			expected:   true,
		},
		{
			name:       "test should match when header name is in lower case",
			headerName: "host",
			regexp:     `^example\.org$`,
			host:       "example.org",
			expected:   true,
		},
		{
			name:       "test should match when header name is in upper case",
			headerName: "HOST",
			regexp:     `^example\.org$`,
			host:       "example.org",
			expected:   true,
		},
		{
			name:       "test shouldn't match when host is empty and regex matches empty value",
			headerName: "Host",
			regexp:     "^$",
			host:       "",
			expected:   false,
		},
		{
			name:       "test shouldn't read the host as a list",
			headerName: "Host",
			regexp:     `^example\.org$`,
			host:       "example.com,example.org",
			expected:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := predicate.NewHeaderPredicate(tt.headerName, tt.regexp)
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test", nil)
			req.Host = tt.host

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}

// net/http moves the Transfer-Encoding header to Request.TransferEncoding and deletes it
// from the headers.
func TestHeaderPredicate_Test_TransferEncoding(t *testing.T) {
	tests := []struct {
		name             string
		headerName       string
		regexp           string
		transferEncoding []string
		expected         bool
	}{
		{
			name:             "test should match when the request is chunked and no regex",
			headerName:       "Transfer-Encoding",
			regexp:           "",
			transferEncoding: []string{"chunked"},
			expected:         true,
		},
		{
			name:             "test shouldn't match when the request has no transfer encoding and no regex",
			headerName:       "Transfer-Encoding",
			regexp:           "",
			transferEncoding: nil,
			expected:         false,
		},
		{
			name:             "test should match when the transfer encoding matches the regex",
			headerName:       "transfer-encoding",
			regexp:           "^chunked$",
			transferEncoding: []string{"chunked"},
			expected:         true,
		},
		{
			name:             "test shouldn't match when the transfer encoding doesn't match the regex",
			headerName:       "Transfer-Encoding",
			regexp:           "^gzip$",
			transferEncoding: []string{"chunked"},
			expected:         false,
		},
		{
			name:             "test shouldn't match when the request has no transfer encoding and regex",
			headerName:       "Transfer-Encoding",
			regexp:           "^chunked$",
			transferEncoding: nil,
			expected:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := predicate.NewHeaderPredicate(tt.headerName, tt.regexp)
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test", nil)
			req.TransferEncoding = tt.transferEncoding

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}

func TestHeaderPredicate_Test_List(t *testing.T) {
	tests := []struct {
		name     string
		regexp   string
		values   []string
		expected bool
	}{
		{
			name:     "test should match when an element of a list in one line matches",
			regexp:   "^v[0-9]+$",
			values:   []string{"beta, v3"},
			expected: true,
		},
		{
			name:     "test should match when the same elements come in two lines",
			regexp:   "^v[0-9]+$",
			values:   []string{"beta", "v3"},
			expected: true,
		},
		{
			name:     "test should match when the first element of a list matches",
			regexp:   "^v[0-9]+$",
			values:   []string{"v3, beta"},
			expected: true,
		},
		{
			name:     "test should match when an element in the middle of a list matches",
			regexp:   "^v[0-9]+$",
			values:   []string{"alpha,v3,beta"},
			expected: true,
		},
		{
			name:     "test shouldn't match when no element of a list matches",
			regexp:   "^v[0-9]+$",
			values:   []string{"alpha, beta, v3x"},
			expected: false,
		},
		{
			name:     "test should match an element of a list in the second line",
			regexp:   "^v[0-9]+$",
			values:   []string{"alpha, beta", "gamma, v3"},
			expected: true,
		},
		{
			name:     "test should trim spaces and tabs around an element",
			regexp:   "^v[0-9]+$",
			values:   []string{"beta \t,\t v3 \t"},
			expected: true,
		},
		{
			name:     "test shouldn't trim whitespace inside an element",
			regexp:   "^v[0-9]+$",
			values:   []string{"beta, v 3"},
			expected: false,
		},
		{
			name:     "test should skip empty elements",
			regexp:   "^v[0-9]+$",
			values:   []string{",, beta, , v3,"},
			expected: true,
		},
		{
			name:     "test shouldn't match an empty element when regex matches empty value",
			regexp:   "^$",
			values:   []string{"beta,, v3, "},
			expected: false,
		},
		{
			name:     "test shouldn't match when the value is only separators and regex matches empty value",
			regexp:   "^$",
			values:   []string{" , ,"},
			expected: false,
		},
		{
			name:     "test should still match the whole value when it has commas and is not a list",
			regexp:   "^Sun, 15 Nov 2026 08:12:31 GMT$",
			values:   []string{"Sun, 15 Nov 2026 08:12:31 GMT"},
			expected: true,
		},
		{
			name:     "test should still match a regex that spans several elements",
			regexp:   "^gzip, *deflate",
			values:   []string{"gzip, deflate, br"},
			expected: true,
		},
		{
			name:     "test should still match the whole value without trimming it",
			regexp:   "^ beta, v3 $",
			values:   []string{" beta, v3 "},
			expected: true,
		},
		{
			name:     "test shouldn't split on a comma inside a quoted string",
			regexp:   `^v[0-9]+"?$`,
			values:   []string{`"beta, v3"`},
			expected: false,
		},
		{
			name:     "test should match a quoted string that has a comma as one element",
			regexp:   `^"beta, v3"$`,
			values:   []string{`alpha, "beta, v3", gamma`},
			expected: true,
		},
		{
			name:     "test should match an element after a quoted string that has a comma",
			regexp:   "^v[0-9]+$",
			values:   []string{`"alpha, beta", v3`},
			expected: true,
		},
		{
			name:     "test should match a parameter with a quoted string that has a comma as part of its element",
			regexp:   `^text/plain; note="a, b"$`,
			values:   []string{`text/html, text/plain; note="a, b"`},
			expected: true,
		},
		{
			name:     "test shouldn't end a quoted string at an escaped quote",
			regexp:   "^v[0-9]+$",
			values:   []string{`"alpha\", v3", beta`},
			expected: false,
		},
		{
			name:     "test should match an element after a quoted string that has an escaped quote",
			regexp:   "^v[0-9]+$",
			values:   []string{`"alpha\", beta", v3`},
			expected: true,
		},
		{
			name:     "test shouldn't take a backslash outside a quoted string as an escape",
			regexp:   "^v[0-9]+$",
			values:   []string{`alpha\, v3`},
			expected: true,
		},
		{
			name:     "test shouldn't split what follows a quote that is never closed",
			regexp:   "^v[0-9]+$",
			values:   []string{`alpha, "beta, v3`},
			expected: false,
		},
		{
			name:     "test should match the element before a quote that is never closed",
			regexp:   "^v[0-9]+$",
			values:   []string{`v3, "beta, alpha`},
			expected: true,
		},
		{
			name:     "test shouldn't match when the value ends in an escape inside a quoted string",
			regexp:   "^v[0-9]+$",
			values:   []string{`alpha, "beta\`},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := predicate.NewHeaderPredicate("X-Api-Version", tt.regexp)
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test", nil)
			req.Header = http.Header{"X-Api-Version": tt.values}

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}

func TestHeaderPredicate_Test_ListWithoutRegexp(t *testing.T) {
	tests := []struct {
		header   http.Header
		name     string
		expected bool
	}{
		{
			name:     "test should match when header is a list of empty elements and no regex",
			header:   http.Header{"X-Api-Version": {" , ,"}},
			expected: true,
		},
		{
			name:     "test shouldn't match when header is not present and no regex",
			header:   http.Header{"X-Other": {"beta, v3"}},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := predicate.NewHeaderPredicate("X-Api-Version", "")
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/server/test", nil)
			req.Header = tt.header

			actual := p.Test(req)
			if tt.expected != actual {
				t.Errorf("expected %t actual %t", tt.expected, actual)
			}
		})
	}
}

func TestHeaderPredicate_Name(t *testing.T) {
	p, _ := predicate.NewHeaderPredicate("X-Test-Id", "1234")

	if p.Name() != predicate.HeaderPredicateName {
		t.Errorf("expected %s actual %s", predicate.HeaderPredicateName, p.Name())
	}
}
