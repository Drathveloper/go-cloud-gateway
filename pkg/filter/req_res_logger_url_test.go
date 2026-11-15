package filter_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// newServerRequest returns a GET request as a server reads it from its request line, which
// is where the URL of a gateway request comes from: the target is the path and the query,
// or an absolute URL.
func newServerRequest(t *testing.T, target string) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
}

// loggedRequestURL runs the pre-process of the filter on the request and returns the "url"
// attribute of the line it logs. The line is decoded rather than searched, because the
// JSON handler escapes the '&' of a query string.
func loggedRequestURL(t *testing.T, f gateway.Filter, req *http.Request) string {
	t.Helper()
	var buf bytes.Buffer
	ctx, cancel := gateway.NewGatewayContext(t.Context(), &gateway.Route{Timeout: time.Minute}, gateway.NewGatewayRequest(req))
	t.Cleanup(cancel)
	ctx.Logger = slog.New(slog.NewJSONHandler(&buf, nil))

	require.NoError(t, f.PreProcess(ctx))

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line), "log line: %s", buf.String())
	logged, isString := line["url"].(string)
	require.True(t, isString, "log line: %s", buf.String())
	return logged
}

// requestLoggers returns the filter in its two variants, which log the URL from different
// places: with the bodies and without them.
func requestLoggers(redactedQueryParams ...string) map[string]*filter.RequestResponseLogger {
	return map[string]*filter.RequestResponseLogger{
		"with bodies": filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true).
			WithRedactedQueryParams(redactedQueryParams...),
		"headers only": filter.NewHeadersOnlyRequestResponseLoggerFilter(slog.LevelInfo).
			WithRedactedQueryParams(redactedQueryParams...),
	}
}

func TestRequestResponseLogger_RedactsSensitiveQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		expected string
		extra    []string
	}{
		{
			name: "log should redact the value when the parameter is one of the default ones",
			target: "/cb?access_token=s1&id_token=s2&refresh_token=s3&token=s4&api_key=s5" +
				"&apikey=s6&password=s7&secret=s8&client_secret=s9&signature=s0",
			expected: "GET /cb?access_token=[REDACTED]&id_token=[REDACTED]&refresh_token=[REDACTED]&token=[REDACTED]" +
				"&api_key=[REDACTED]&apikey=[REDACTED]&password=[REDACTED]&secret=[REDACTED]" +
				"&client_secret=[REDACTED]&signature=[REDACTED]",
		},
		{
			name:     "log should redact the value and keep the name as written when the name is in another case",
			target:   "/cb?Access_Token=abc&TOKEN=def&ApiKey=ghi",
			expected: "GET /cb?Access_Token=[REDACTED]&TOKEN=[REDACTED]&ApiKey=[REDACTED]",
		},
		{
			name:     "log should redact every value when the parameter is repeated",
			target:   "/cb?token=one&page=1&token=two&token=three",
			expected: "GET /cb?token=[REDACTED]&page=1&token=[REDACTED]&token=[REDACTED]",
		},
		{
			name:     "log should keep the parameter as it is when it has no value",
			target:   "/cb?token&page=1&secret",
			expected: "GET /cb?token&page=1&secret",
		},
		{
			name:     "log should keep the parameter as it is when its value is empty",
			target:   "/cb?token=&page=1&secret=",
			expected: "GET /cb?token=&page=1&secret=",
		},
		{
			name:     "log should keep the url as it is when there is no query",
			target:   "/cb",
			expected: "GET /cb",
		},
		{
			name:     "log should keep the url as it is when the query is empty",
			target:   "/cb?",
			expected: "GET /cb?",
		},
		{
			name:     "log should keep the url as it is when no parameter is sensitive",
			target:   "/cb?b=%7e&a=x+y&c&a=1",
			expected: "GET /cb?b=%7e&a=x+y&c&a=1",
		},
		{
			name:     "log should keep the parameter as it is when its name only contains a sensitive one",
			target:   "/cb?tokens=abc&mytoken=abc&token_type=bearer&access_token_hint=abc&to=ken",
			expected: "GET /cb?tokens=abc&mytoken=abc&token_type=bearer&access_token_hint=abc&to=ken",
		},
		{
			name:     "log should redact the value when the parameters are separated by semicolons",
			target:   "/cb?a=1;token=abc;b=2&c=3;secret=def",
			expected: "GET /cb?a=1;token=[REDACTED];b=2&c=3;secret=[REDACTED]",
		},
		{
			name:     "log should keep the malformed pairs as they are when a parameter among them is sensitive",
			target:   "/cb?&&=x&%zz=1&token=abc&=&a==b&%=%&;&",
			expected: "GET /cb?&&=x&%zz=1&token=[REDACTED]&=&a==b&%=%&;&",
		},
		{
			name:     "log should redact the value and keep the name encoded when the name is percent-encoded",
			target:   "/cb?access%5Ftoken=abc&%74oken=def&api%5fkey=ghi&%53%45%43%52%45%54=jkl",
			expected: "GET /cb?access%5Ftoken=[REDACTED]&%74oken=[REDACTED]&api%5fkey=[REDACTED]&%53%45%43%52%45%54=[REDACTED]",
		},
		{
			name:     "log should keep the parameter as it is when its name has an escape that is not valid",
			target:   "/cb?token%=abc&tok%zzen=def&token%5=ghi",
			expected: "GET /cb?token%=abc&tok%zzen=def&token%5=ghi",
		},
		{
			name:     "log should keep the parameter as it is when its name is encoded twice",
			target:   "/cb?access%255Ftoken=abc",
			expected: "GET /cb?access%255Ftoken=abc",
		},
		{
			name:     "log should keep the order of the parameters when some of them are sensitive",
			target:   "/cb?z=1&token=abc&a=2&secret=def&m=3",
			expected: "GET /cb?z=1&token=[REDACTED]&a=2&secret=[REDACTED]&m=3",
		},
		{
			name:     "log should redact the whole value when the value contains equal signs",
			target:   "/cb?token=abc=def==&page=1&signature=a=b",
			expected: "GET /cb?token=[REDACTED]&page=1&signature=[REDACTED]",
		},
		{
			name:     "log should redact the whole value when the value is encoded",
			target:   "/cb?password=p%40ss+w%26rd%3Bx&page=1",
			expected: "GET /cb?password=[REDACTED]&page=1",
		},
		{
			name:     "log should redact the value when the sensitive parameter is the only one",
			target:   "/cb?token=abc",
			expected: "GET /cb?token=[REDACTED]",
		},
		{
			name:     "log should redact the added parameters and the default ones when parameters are added",
			target:   "/cb?session_id=abc&SESSION_ID=def&api+key=ghi&api%20key=jkl&token=mno&page=1",
			expected: "GET /cb?session_id=[REDACTED]&SESSION_ID=[REDACTED]&api+key=[REDACTED]&api%20key=[REDACTED]&token=[REDACTED]&page=1",
			extra:    []string{" Session_Id ", "api key"},
		},
		{
			// Only the case of the ASCII letters is ignored: "%C3%91" is the upper case of "ñ".
			name:     "log should redact the added parameter when its name is not ascii and only differs in the case of its ascii letters",
			target:   "/cb?contrase%C3%B1a=abc&CONTRASE%c3%b1A=def&CONTRASE%C3%91A=visible",
			expected: "GET /cb?contrase%C3%B1a=[REDACTED]&CONTRASE%c3%b1A=[REDACTED]&CONTRASE%C3%91A=visible",
			extra:    []string{"contraseña"},
		},
		{
			name:     "log should keep the added parameter as it is when it is not written as the backend reads it",
			target:   "/cb?api+key=abc&api%20key=def",
			expected: "GET /cb?api+key=abc&api%20key=def",
			extra:    []string{"api+key", "api%20key"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for variant, f := range requestLoggers(tt.extra...) {
				t.Run(variant, func(t *testing.T) {
					req := newServerRequest(t, tt.target)
					liveQuery := req.URL.RawQuery

					actual := loggedRequestURL(t, f, req)

					assert.Equal(t, tt.expected, actual)
					// The live URL is the one the request to the backend is built from.
					assert.Equal(t, liveQuery, req.URL.RawQuery)
					assert.Equal(t, tt.target, req.URL.String())
				})
			}
		})
	}
}

func TestRequestResponseLogger_RedactsSensitiveQueryParams_ZeroValue(t *testing.T) {
	req := newServerRequest(t, "/cb?access_token=abc&page=1")

	actual := loggedRequestURL(t, &filter.RequestResponseLogger{}, req)

	assert.Equal(t, "GET /cb?access_token=[REDACTED]&page=1", actual)
}

func TestRequestResponseLogger_RedactsUserInfo(t *testing.T) {
	// The target is "http://" followed by the user info and by the rest. They are kept apart
	// because a literal URL with a password in it is reported as a hardcoded credential.
	tests := []struct {
		name     string
		userInfo string
		rest     string
		expected string
	}{
		{
			name:     "log should redact the user info when the url has a user and a password",
			userInfo: "user:pass@",
			rest:     "example.org/cb?page=1",
			expected: "GET http://[REDACTED]@example.org/cb?page=1",
		},
		{
			name:     "log should redact the user info when the url only has a user",
			userInfo: "token-as-user@",
			rest:     "example.org/cb",
			expected: "GET http://[REDACTED]@example.org/cb",
		},
		{
			name:     "log should redact the user info when the url has a user and an empty password",
			userInfo: "user:@",
			rest:     "example.org/cb",
			expected: "GET http://[REDACTED]@example.org/cb",
		},
		{
			name:     "log should redact the user info and the query when both carry credentials",
			userInfo: "user:pass@",
			rest:     "example.org:8080/cb?token=abc&page=1",
			expected: "GET http://[REDACTED]@example.org:8080/cb?token=[REDACTED]&page=1",
		},
		{
			name:     "log should only redact the user info when the rest of the url has at signs",
			userInfo: "us%40er:p%40ss@",
			rest:     "example.org/a@b?mail=a@b",
			expected: "GET http://[REDACTED]@example.org/a@b?mail=a@b",
		},
		{
			name:     "log should keep the url as it is when it is absolute and has no user info",
			userInfo: "",
			rest:     "example.org/a@b?mail=a@b",
			expected: "GET http://example.org/a@b?mail=a@b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for variant, f := range requestLoggers() {
				t.Run(variant, func(t *testing.T) {
					target := "http://" + tt.userInfo + tt.rest
					req := newServerRequest(t, target)

					actual := loggedRequestURL(t, f, req)

					assert.Equal(t, tt.expected, actual)
					assert.Equal(t, target, req.URL.String())
				})
			}
		})
	}
}

// A server request has user info when its request line holds an absolute URL. No browser
// writes one, but any client can, so this goes through a real server instead of a request
// built by hand: it is what shows that the user info reaches the filter at all.
func TestRequestResponseLogger_RedactsUserInfo_OverTheWire(t *testing.T) {
	f := filter.NewHeadersOnlyRequestResponseLoggerFilter(slog.LevelInfo)
	// received is the URL the server hands to the handler, logged is the line of the filter.
	received := make(chan string, 1)
	logged := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		var buf bytes.Buffer
		route := &gateway.Route{Timeout: time.Minute, Logger: slog.New(slog.NewTextHandler(&buf, nil))}
		ctx, cancel := gateway.NewGatewayContext(req.Context(), route, gateway.NewGatewayRequest(req))
		defer cancel()
		_ = f.PreProcess(ctx)
		received <- req.URL.String()
		logged <- buf.String()
	}))
	defer server.Close()
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", server.Listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck

	_, err = fmt.Fprint(conn, "GET http://user:pass@example.org/cb?token=abc HTTP/1.1\r\nHost: example.org\r\nConnection: close\r\n\r\n")
	require.NoError(t, err)
	// The server closes the connection after the response, as the request asks.
	response, err := io.ReadAll(conn)
	require.NoError(t, err)

	require.Contains(t, string(response), "HTTP/1.1 200 OK\r\n")
	assert.Equal(t, "http://user:pass@example.org/cb?token=abc", <-received)
	line := <-logged
	assert.Contains(t, line, `url="GET http://[REDACTED]@example.org/cb?token=[REDACTED]"`)
	assert.NotContains(t, line, "pass")
}

// queryRecorder is a filter that records the URL of the request it is given.
type queryRecorder struct {
	seen string
}

func (r *queryRecorder) PreProcess(ctx *gateway.Context) error {
	r.seen = ctx.Request.URL.String()
	return nil
}

func (r *queryRecorder) PostProcess(_ *gateway.Context) error { return nil }

func (r *queryRecorder) Name() string { return "QueryRecorder" }

// backendRecorder is an http client that records the URL of the request sent to the backend.
type backendRecorder struct {
	seen string
}

func (c *backendRecorder) Do(req *http.Request) (*http.Response, error) {
	c.seen = req.URL.String()
	return &http.Response{StatusCode: http.StatusNoContent, Header: http.Header{}, Body: http.NoBody}, nil
}

func TestRequestResponseLogger_LeavesTheQueryOfTheLiveRequestUntouched(t *testing.T) {
	const target = "/cb?z=1&access_token=s3cr3t-one&Token=s3cr3t-two&a=2"
	for variant, f := range requestLoggers() {
		t.Run(variant, func(t *testing.T) {
			var buf bytes.Buffer
			nextFilter := &queryRecorder{}
			backend := &backendRecorder{}
			route, err := gateway.NewRoute("r1", "http://backend.internal:8080", nil, nil,
				gateway.Filters{f, nextFilter}, time.Minute, nil, slog.New(slog.NewTextHandler(&buf, nil)))
			require.NoError(t, err)
			req := newServerRequest(t, target)
			ctx, cancel := gateway.NewGatewayContext(t.Context(), route, gateway.NewGatewayRequest(req))
			defer cancel()

			require.NoError(t, gateway.NewGateway(backend).Do(ctx))
			_, err = io.Copy(io.Discard, ctx.Response.BodyReader)
			require.NoError(t, err)

			// The log is the only place where the credentials are replaced.
			assert.Contains(t, buf.String(), `url="GET /cb?z=1&access_token=[REDACTED]&Token=[REDACTED]&a=2"`)
			assert.NotContains(t, buf.String(), "s3cr3t")
			assert.Equal(t, target, nextFilter.seen)
			assert.Equal(t, "http://backend.internal:8080"+target, backend.seen)
			assert.Equal(t, target, req.URL.String())
		})
	}
}

func TestRequestResponseLoggerBuilder_RedactQueryParams(t *testing.T) {
	f, err := filter.NewRequestResponseLoggerBuilder().Build(map[string]any{
		"redact-query-params": []any{"Session_Id", "sig"},
	})
	require.NoError(t, err)
	req := newServerRequest(t, "/cb?session_id=abc&sig=def&token=ghi&page=1")

	actual := loggedRequestURL(t, f, req)

	assert.Equal(t, "GET /cb?session_id=[REDACTED]&sig=[REDACTED]&token=[REDACTED]&page=1", actual)
}

func TestRequestResponseLoggerBuilder_RedactQueryParams_KeepsTheRedactedHeaders(t *testing.T) {
	var buf bytes.Buffer
	headers := http.Header{"X-Custom-Token": {"custom-secret"}, "Accept": {"text/plain"}}
	ctx := newJSONLoggedRequestContext(t, &buf, headers, nil)

	f, err := filter.NewRequestResponseLoggerBuilder().Build(map[string]any{
		"redact-headers":      []any{"x-custom-token"},
		"redact-query-params": []any{"sig"},
	})
	require.NoError(t, err)
	require.NoError(t, f.PreProcess(ctx))

	assert.Contains(t, buf.String(), `"X-Custom-Token":["[REDACTED]"]`)
	assert.NotContains(t, buf.String(), "custom-secret")
}

func TestRequestResponseLogger_WithRedactedQueryParams_LeavesReceiverUntouched(t *testing.T) {
	base := filter.NewHeadersOnlyRequestResponseLoggerFilter(slog.LevelInfo).WithRedactedHeaders("X-Custom-Token")

	derived := base.WithRedactedQueryParams("sig")

	assert.NotSame(t, base, derived)
	assert.Equal(t, "GET /cb?sig=visible",
		loggedRequestURL(t, base, newServerRequest(t, "/cb?sig=visible")))
	assert.Equal(t, "GET /cb?sig=[REDACTED]",
		loggedRequestURL(t, derived, newServerRequest(t, "/cb?sig=hidden")))
}

func TestRequestResponseLogger_WithRedactedQueryParams_AddsToThePreviousOnes(t *testing.T) {
	first := filter.NewHeadersOnlyRequestResponseLoggerFilter(slog.LevelInfo).WithRedactedQueryParams("sig")

	second := first.WithRedactedQueryParams("session_id")
	// Appending to the names of the first filter must not write into the ones of the second.
	third := first.WithRedactedQueryParams("other")
	req := newServerRequest(t, "/cb?sig=abc&session_id=def&other=ghi")

	assert.Equal(t, "GET /cb?sig=[REDACTED]&session_id=def&other=ghi", loggedRequestURL(t, first, req))
	assert.Equal(t, "GET /cb?sig=[REDACTED]&session_id=[REDACTED]&other=ghi", loggedRequestURL(t, second, req))
	assert.Equal(t, "GET /cb?sig=[REDACTED]&session_id=def&other=[REDACTED]", loggedRequestURL(t, third, req))
}

// The header and the query parameter sets are independent: adding to one keeps the other.
func TestRequestResponseLogger_WithRedactedHeaders_KeepsTheRedactedQueryParams(t *testing.T) {
	var buf bytes.Buffer
	req := newServerRequest(t, "/cb?sig=query-secret")
	req.Header.Set("X-Custom-Token", "header-secret")
	ctx, cancel := gateway.NewGatewayContext(t.Context(), &gateway.Route{Timeout: time.Minute}, gateway.NewGatewayRequest(req))
	defer cancel()
	ctx.Logger = slog.New(slog.NewTextHandler(&buf, nil))

	f := filter.NewRequestResponseLoggerFilter(slog.LevelWarn, 16, false).
		WithRedactedQueryParams("sig").
		WithRedactedHeaders("X-Custom-Token")
	require.NoError(t, f.PreProcess(ctx))

	// The level and the body settings travel with the copies as well.
	assert.Contains(t, buf.String(), "level=WARN")
	assert.NotContains(t, buf.String(), "body=")
	assert.Contains(t, buf.String(), `url="GET /cb?sig=[REDACTED]"`)
	assert.Contains(t, buf.String(), "X-Custom-Token:[[REDACTED]]")
	assert.NotContains(t, buf.String(), "secret")
}
