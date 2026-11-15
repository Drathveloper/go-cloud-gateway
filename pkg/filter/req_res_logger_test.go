package filter_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func TestNewRequestResponseLoggerFilterBuilder(t *testing.T) {
	tests := []struct {
		expectedErr error
		args        map[string]any
		name        string
	}{
		{
			name:        "build should succeed when no args present",
			args:        map[string]any{},
			expectedErr: nil,
		},
		{
			name: "build should succeed when level is present and is debug",
			args: map[string]any{
				"level": "DEBUG",
			},
			expectedErr: nil,
		},
		{
			name: "build should succeed when level is present and is info",
			args: map[string]any{
				"level": "INFO",
			},
			expectedErr: nil,
		},
		{
			name: "build should succeed when level is present and is warn",
			args: map[string]any{
				"level": "WARN",
			},
			expectedErr: nil,
		},
		{
			name: "build should succeed when level is present and is error",
			args: map[string]any{
				"level": "ERROR",
			},
			expectedErr: nil,
		},
		{
			name: "build should fail when level is present and names no level",
			args: map[string]any{
				"level": "warning",
			},
			expectedErr: errors.New("invalid log level \"warning\": expected debug, info, warn or error"),
		},
		{
			name: "build should fail when level is present and is empty",
			args: map[string]any{
				"level": "",
			},
			expectedErr: errors.New("invalid log level \"\": expected debug, info, warn or error"),
		},
		{
			name: "build should fail when level is present and is not a string",
			args: map[string]any{
				"level": 42,
			},
			expectedErr: errors.New("failed to convert 'level' attribute: value is required to be a valid string"),
		},
		{
			name: "build should succeed when level is present in another case",
			args: map[string]any{
				"level": "WARN",
			},
			expectedErr: nil,
		},
		{
			name: "build should succeed when level is null",
			args: map[string]any{
				"level": nil,
			},
			expectedErr: nil,
		},
		{
			name: "build should succeed when log-bodies is present and is bool",
			args: map[string]any{
				"log-bodies": false,
			},
			expectedErr: nil,
		},
		{
			name: "build should fail when log-bodies is present and is not bool",
			args: map[string]any{
				"log-bodies": 42,
			},
			expectedErr: errors.New("failed to convert 'log-bodies' attribute: value is required to be a valid bool"),
		},
		{
			name: "build should succeed when redact-headers is present and is a list of names",
			args: map[string]any{
				"redact-headers": []any{"X-Custom-Token", "x-other-token"},
			},
			expectedErr: nil,
		},
		{
			name: "build should fail when redact-headers is present and is not a list",
			args: map[string]any{
				"redact-headers": "X-Custom-Token",
			},
			expectedErr: errors.New("failed to convert 'redact-headers' attribute: value is required to be a valid slice"),
		},
		{
			name: "build should fail when redact-headers is present and holds something other than names",
			args: map[string]any{
				"redact-headers": []any{"X-Custom-Token", 42},
			},
			expectedErr: errors.New("failed to convert 'redact-headers' attribute: value is expected to be a valid string slice: " +
				"value is required to be a valid slice: element at index 1 is not of expected type"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := filter.NewRequestResponseLoggerBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestRequestResponseLogger_Name(t *testing.T) {
	expected := "RequestResponseLogger"

	f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, false)

	actual := f.Name()

	if expected != actual {
		t.Errorf("expected %s actual %s", expected, actual)
	}
}

func TestRequestResponseLogger_PreProcess(t *testing.T) {
	tests := []struct {
		headers  http.Header
		name     string
		path     string
		method   string
		expected string
		body     []byte
	}{
		{
			name:     "log request should succeed when body is empty",
			path:     "/test?page=1&size=10",
			method:   "GET",
			headers:  map[string][]string{"Accept-Language": {"h1", "v1"}},
			body:     nil,
			expected: "level=INFO msg=\"Received request\" url=\"GET https://example.org//test?page=1&size=10\" headers=\"map[Accept-Language:[h1 v1]]\" body=\"\"",
		},
		{
			name:     "log request should succeed when body is present",
			path:     "/test?page=1&size=10",
			method:   "POST",
			headers:  map[string][]string{"Accept-Language": {"h1", "v1"}},
			body:     []byte("{\"k1\":\"abc\"}"),
			expected: "level=INFO msg=\"Received request\" url=\"POST https://example.org//test?page=1&size=10\" headers=\"map[Accept-Language:[h1 v1]]\" body=\"{\\\"k1\\\":\\\"abc\\\"}\"",
		},
		{
			name:     "log request should succeed when headers are empty",
			path:     "/test?page=1&size=10",
			method:   "POST",
			headers:  nil,
			body:     []byte("{\"k1\":\"abc\"}"),
			expected: "level=INFO msg=\"Received request\" url=\"POST https://example.org//test?page=1&size=10\" headers=map[] body=\"{\\\"k1\\\":\\\"abc\\\"}\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))
			req, _ := http.NewRequestWithContext(t.Context(), tt.method, "https://example.org/"+tt.path, bytes.NewBuffer(tt.body))
			req.Header = tt.headers
			gwReq := gateway.NewGatewayRequest(req)
			ctx, _ := gateway.NewGatewayContext(t.Context(), &gateway.Route{}, gwReq)
			ctx.Logger = logger

			f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true)
			_ = f.PreProcess(ctx)

			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("expected: %s\nactual: %s", tt.expected, buf.String())
			}
		})
	}
}

func TestRequestResponseLogger_BodyOverLimitIsForwardedUntouched(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 1024)
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.org/test", bytes.NewReader(payload))
	gwReq := gateway.NewGatewayRequest(req)
	ctx, _ := gateway.NewGatewayContext(t.Context(), &gateway.Route{}, gwReq)
	ctx.Logger = logger

	f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo, 16, false)
	if err := f.PreProcess(ctx); err != nil {
		t.Fatalf("pre-process failed: %v", err)
	}

	if strings.Contains(buf.String(), "aaaaaaaa") {
		t.Error("expected the over-limit body to be omitted from the log")
	}
	got, err := io.ReadAll(ctx.Request.BodyReader)
	if err != nil {
		t.Fatalf("reading forwarded body failed: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("expected the body to remain fully forwardable, got %d bytes want %d", len(got), len(payload))
	}
}

func TestRequestResponseLogger_HeadersOnlySkipsBodies(t *testing.T) {
	payload := []byte("{\"k1\":\"abc\"}")
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.org/test", bytes.NewReader(payload))
	gwReq := gateway.NewGatewayRequest(req)
	ctx, _ := gateway.NewGatewayContext(t.Context(), &gateway.Route{}, gwReq)
	ctx.Logger = logger
	res := &http.Response{
		StatusCode: http.StatusOK,
		Header:     map[string][]string{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(payload)),
	}
	ctx.Response = gateway.NewGatewayResponse(res)

	f := filter.NewHeadersOnlyRequestResponseLoggerFilter(slog.LevelInfo)
	if err := f.PreProcess(ctx); err != nil {
		t.Fatalf("pre-process failed: %v", err)
	}
	if err := f.PostProcess(ctx); err != nil {
		t.Fatalf("post-process failed: %v", err)
	}

	logged := buf.String()
	if !strings.Contains(logged, "Received request") || !strings.Contains(logged, "Returned response") {
		t.Fatalf("expected both log lines, got: %s", logged)
	}
	if strings.Contains(logged, "body=") {
		t.Errorf("expected no body attribute in headers-only mode, got: %s", logged)
	}
	if strings.Contains(logged, "k1") {
		t.Errorf("expected body content to be absent from the log, got: %s", logged)
	}
	// The bodies must not have been captured: both must remain readable as streams.
	gotReq, err := io.ReadAll(ctx.Request.BodyReader)
	if err != nil {
		t.Fatalf("reading request body failed: %v", err)
	}
	if !bytes.Equal(gotReq, payload) {
		t.Errorf("expected request body to remain forwardable, got %q", gotReq)
	}
	gotRes, err := io.ReadAll(ctx.Response.BodyReader)
	if err != nil {
		t.Fatalf("reading response body failed: %v", err)
	}
	if !bytes.Equal(gotRes, payload) {
		t.Errorf("expected response body to remain forwardable, got %q", gotRes)
	}
}

func TestRequestResponseLogger_PostProcess(t *testing.T) {
	tests := []struct {
		headers  http.Header
		name     string
		expected string
		body     []byte
		status   int
	}{
		{
			name:     "log response should succeed when body is empty",
			status:   http.StatusOK,
			headers:  map[string][]string{"Accept-Language": {"h1", "v1"}},
			body:     nil,
			expected: "Returned response\" status=200 headers=\"map[Accept-Language:[h1 v1]]\" body=\"\"",
		},
		{
			name:     "log request should succeed when body is present",
			status:   http.StatusOK,
			headers:  map[string][]string{"Accept-Language": {"h1", "v1"}},
			body:     []byte("{\"k1\":\"abc\"}"),
			expected: "Returned response\" status=200 headers=\"map[Accept-Language:[h1 v1]]\" body=\"{\\\"k1\\\":\\\"abc\\\"}\"",
		},
		{
			name:     "log request should succeed when headers are empty",
			status:   http.StatusOK,
			headers:  nil,
			body:     []byte("{\"k1\":\"abc\"}"),
			expected: "Returned response\" status=200 headers=map[] body=\"{\\\"k1\\\":\\\"abc\\\"}\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))
			var bodyBytes io.ReadCloser
			contentLength := int64(0)
			if tt.body != nil {
				bodyBytes = io.NopCloser(bytes.NewBuffer(tt.body))
				contentLength = int64(len(tt.body))
			}
			res := &http.Response{
				StatusCode:    tt.status,
				Header:        tt.headers,
				Body:          bodyBytes,
				ContentLength: contentLength,
			}
			gwRes := gateway.NewGatewayResponse(res)
			ctx, _ := gateway.NewGatewayContext(t.Context(), &gateway.Route{}, nil)
			ctx.Logger = logger
			ctx.Response = gwRes

			f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true)
			_ = f.PostProcess(ctx)
			// The response is logged when its body finishes streaming to the client:
			// drain it as writeResponse would to trigger the log line.
			if _, err := io.Copy(io.Discard, ctx.Response.BodyReader); err != nil {
				t.Fatalf("draining response body failed: %v", err)
			}

			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("expected: %s\nactual: %s", tt.expected, buf.String())
			}
		})
	}
}

func newLoggedResponseContext(t *testing.T, buf *bytes.Buffer, body []byte, length int64) *gateway.Context {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(buf, nil))
	res := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(body)), length)
	ctx, _ := gateway.NewGatewayContext(t.Context(), &gateway.Route{}, nil)
	ctx.Logger = logger
	ctx.Response = &gateway.Response{
		Status:     http.StatusOK,
		Headers:    http.Header{"Content-Type": {"text/event-stream"}},
		BodyReader: res,
	}
	return ctx
}

func TestRequestResponseLogger_PostProcess_DoesNotBufferStream(t *testing.T) {
	payload := []byte("event: one\n\nevent: two\n\n")
	var buf bytes.Buffer
	ctx := newLoggedResponseContext(t, &buf, payload, -1)

	f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true)
	if err := f.PostProcess(ctx); err != nil {
		t.Fatalf("post-process failed: %v", err)
	}
	// The body has not streamed yet, so nothing is logged: PostProcess did not buffer it.
	if strings.Contains(buf.String(), "Returned response") {
		t.Fatalf("expected no response log before the body streamed, got: %s", buf.String())
	}
	// Streaming to the client both forwards the body untouched and triggers the log.
	got, err := io.ReadAll(ctx.Response.BodyReader)
	if err != nil {
		t.Fatalf("streaming response body failed: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("expected body to remain fully forwardable, got %q", got)
	}
	logged := buf.String()
	if !strings.Contains(logged, "Returned response") || !strings.Contains(logged, "event: one") {
		t.Errorf("expected the response logged with its body after streaming, got: %s", logged)
	}
	if !strings.Contains(logged, "bytes=24") {
		t.Errorf("expected the total byte count logged, got: %s", logged)
	}
}

func TestRequestResponseLogger_PostProcess_BodyOverLimitLogsTruncatedPrefix(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 1024)
	var buf bytes.Buffer
	ctx := newLoggedResponseContext(t, &buf, payload, int64(len(payload)))

	f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo, 16, true)
	if err := f.PostProcess(ctx); err != nil {
		t.Fatalf("post-process failed: %v", err)
	}
	got, err := io.ReadAll(ctx.Response.BodyReader)
	if err != nil {
		t.Fatalf("streaming response body failed: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("expected over-limit body to remain fully forwardable, got %d bytes want %d", len(got), len(payload))
	}
	logged := buf.String()
	// The logged prefix is capped at the limit, but the true size is reported. The text
	// handler does not quote a string that needs no quoting.
	if !strings.Contains(logged, "body=aaaaaaaaaaaaaaaa bytes=") {
		t.Errorf("expected a 16-byte truncated prefix logged, got: %s", logged)
	}
	if strings.Contains(logged, strings.Repeat("a", 17)) {
		t.Errorf("expected the logged body capped at 16 bytes, got: %s", logged)
	}
	if !strings.Contains(logged, "bytes=1024") {
		t.Errorf("expected the true byte count logged, got: %s", logged)
	}
}

func TestRequestResponseLogger_PostProcess_TruncatedStreamLogsError(t *testing.T) {
	var buf bytes.Buffer
	ctx := newLoggedResponseContext(t, &buf, []byte("partial event"), -1)

	f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true)
	if err := f.PostProcess(ctx); err != nil {
		t.Fatalf("post-process failed: %v", err)
	}
	// Read part of the stream, then close it before EOF (client disconnect / pipeline error).
	if _, err := ctx.Response.BodyReader.Read(make([]byte, 4)); err != nil {
		t.Fatalf("partial read failed: %v", err)
	}
	if err := ctx.Response.BodyReader.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	logged := buf.String()
	if !strings.Contains(logged, "Returned response") {
		t.Fatalf("expected the response logged on truncation, got: %s", logged)
	}
	if !strings.Contains(logged, "body stream closed before EOF") {
		t.Errorf("expected the truncation error logged, got: %s", logged)
	}
}

func newJSONLoggedRequestContext(t *testing.T, buf *bytes.Buffer, headers http.Header, body []byte) *gateway.Context {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.org/test", bytes.NewReader(body))
	req.Header = headers
	ctx, _ := gateway.NewGatewayContext(t.Context(), &gateway.Route{}, gateway.NewGatewayRequest(req))
	ctx.Logger = slog.New(slog.NewJSONHandler(buf, nil))
	return ctx
}

func newJSONLoggedResponseContext(t *testing.T, buf *bytes.Buffer, headers http.Header, body []byte) *gateway.Context {
	t.Helper()
	ctx, _ := gateway.NewGatewayContext(t.Context(), &gateway.Route{}, nil)
	ctx.Logger = slog.New(slog.NewJSONHandler(buf, nil))
	ctx.Response = &gateway.Response{
		Status:     http.StatusOK,
		Headers:    headers,
		BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(body)), int64(len(body))),
	}
	return ctx
}

func TestRequestResponseLogger_RedactsSensitiveHeaders(t *testing.T) {
	tests := []struct {
		filter     *filter.RequestResponseLogger
		headers    http.Header
		name       string
		expected   []string
		unexpected []string
	}{
		{
			name:   "log should redact every default sensitive header when bodies are logged",
			filter: filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true),
			headers: http.Header{
				"Authorization":       {"Bearer super-secret"},
				"Proxy-Authorization": {"Basic proxy-secret"},
				"Cookie":              {"session=cookie-secret"},
				"Set-Cookie":          {"session=set-cookie-secret"},
				"X-Api-Key":           {"api-key-secret"},
				"Accept":              {"text/plain"},
			},
			expected: []string{
				`"Authorization":["[REDACTED]"]`,
				`"Proxy-Authorization":["[REDACTED]"]`,
				`"Cookie":["[REDACTED]"]`,
				`"Set-Cookie":["[REDACTED]"]`,
				`"X-Api-Key":["[REDACTED]"]`,
				`"Accept":["text/plain"]`,
			},
			unexpected: []string{"secret"},
		},
		{
			name:   "log should redact every default sensitive header when only headers are logged",
			filter: filter.NewHeadersOnlyRequestResponseLoggerFilter(slog.LevelInfo),
			headers: http.Header{
				"Authorization": {"Bearer super-secret"},
				"Set-Cookie":    {"session=set-cookie-secret"},
				"Accept":        {"text/plain"},
			},
			expected: []string{
				`"Authorization":["[REDACTED]"]`,
				`"Set-Cookie":["[REDACTED]"]`,
				`"Accept":["text/plain"]`,
			},
			unexpected: []string{"secret"},
		},
		{
			name:   "log should redact every default sensitive header when the filter is the zero value",
			filter: &filter.RequestResponseLogger{},
			headers: http.Header{
				"Authorization": {"Bearer super-secret"},
			},
			expected:   []string{`"Authorization":["[REDACTED]"]`},
			unexpected: []string{"secret"},
		},
		{
			name:   "log should redact each value when a sensitive header has several",
			filter: filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true),
			headers: http.Header{
				"Set-Cookie": {"a=first-secret", "b=second-secret"},
			},
			expected:   []string{`"Set-Cookie":["[REDACTED]","[REDACTED]"]`},
			unexpected: []string{"secret"},
		},
		{
			name:   "log should redact a sensitive header when its key is not canonical",
			filter: filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true),
			headers: http.Header{
				"authorization": {"Bearer super-secret"},
				"X-API-KEY":     {"api-key-secret"},
			},
			expected: []string{
				`"authorization":["[REDACTED]"]`,
				`"X-API-KEY":["[REDACTED]"]`,
			},
			unexpected: []string{"secret"},
		},
		{
			name: "log should redact the added headers and the default ones when headers are added",
			filter: filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true).
				WithRedactedHeaders("x-custom-token", " X-Other-Token "),
			headers: http.Header{
				"X-Custom-Token": {"custom-secret"},
				"X-Other-Token":  {"other-secret"},
				"Authorization":  {"Bearer super-secret"},
				"Accept":         {"text/plain"},
			},
			expected: []string{
				`"X-Custom-Token":["[REDACTED]"]`,
				`"X-Other-Token":["[REDACTED]"]`,
				`"Authorization":["[REDACTED]"]`,
				`"Accept":["text/plain"]`,
			},
			unexpected: []string{"secret"},
		},
		{
			name:   "log should keep the headers as they are when none is sensitive",
			filter: filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true),
			headers: http.Header{
				"X-Custom-Token": {"not-redacted-by-default"},
				"Accept":         {"text/plain"},
			},
			expected: []string{
				`"X-Custom-Token":["not-redacted-by-default"]`,
				`"Accept":["text/plain"]`,
			},
			unexpected: []string{"REDACTED"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sides := []struct {
				log  func(buf *bytes.Buffer, headers http.Header)
				name string
			}{
				{
					name: "request",
					log: func(buf *bytes.Buffer, headers http.Header) {
						ctx := newJSONLoggedRequestContext(t, buf, headers, []byte("payload"))
						_ = tt.filter.PreProcess(ctx)
					},
				},
				{
					name: "response",
					log: func(buf *bytes.Buffer, headers http.Header) {
						ctx := newJSONLoggedResponseContext(t, buf, headers, []byte("payload"))
						_ = tt.filter.PostProcess(ctx)
						_, _ = io.Copy(io.Discard, ctx.Response.BodyReader)
					},
				},
			}
			for _, side := range sides {
				t.Run(side.name, func(t *testing.T) {
					var buf bytes.Buffer
					// The live headers are the ones forwarded: logging must leave them intact.
					live := tt.headers.Clone()

					side.log(&buf, live)

					for _, expected := range tt.expected {
						if !strings.Contains(buf.String(), expected) {
							t.Errorf("expected: %s\nactual: %s", expected, buf.String())
						}
					}
					for _, unexpected := range tt.unexpected {
						if strings.Contains(buf.String(), unexpected) {
							t.Errorf("expected %s to be absent\nactual: %s", unexpected, buf.String())
						}
					}
					if !reflect.DeepEqual(tt.headers, live) {
						t.Errorf("expected %v actual %v", tt.headers, live)
					}
				})
			}
		})
	}
}

func TestRequestResponseLoggerBuilder_RedactHeaders(t *testing.T) {
	var buf bytes.Buffer
	headers := http.Header{
		"X-Custom-Token": {"custom-secret"},
		"Authorization":  {"Bearer super-secret"},
		"Accept":         {"text/plain"},
	}
	ctx := newJSONLoggedRequestContext(t, &buf, headers, nil)

	f, err := filter.NewRequestResponseLoggerBuilder().Build(map[string]any{
		"redact-headers": []any{"x-custom-token"},
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	_ = f.PreProcess(ctx)

	expected := []string{
		`"X-Custom-Token":["[REDACTED]"]`,
		`"Authorization":["[REDACTED]"]`,
		`"Accept":["text/plain"]`,
	}
	for _, fragment := range expected {
		if !strings.Contains(buf.String(), fragment) {
			t.Errorf("expected: %s\nactual: %s", fragment, buf.String())
		}
	}
	if strings.Contains(buf.String(), "secret") {
		t.Errorf("expected the secrets to be absent\nactual: %s", buf.String())
	}
}

func TestRequestResponseLogger_WithRedactedHeaders_LeavesReceiverUntouched(t *testing.T) {
	var buf bytes.Buffer
	ctx := newJSONLoggedRequestContext(t, &buf, http.Header{"X-Custom-Token": {"visible"}}, nil)
	base := filter.NewHeadersOnlyRequestResponseLoggerFilter(slog.LevelInfo)

	derived := base.WithRedactedHeaders("X-Custom-Token")
	_ = base.PreProcess(ctx)

	if derived == base {
		t.Error("expected a new filter to be returned")
	}
	expected := `"X-Custom-Token":["visible"]`
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected: %s\nactual: %s", expected, buf.String())
	}
}

func TestRequestResponseLogger_DisabledLevelDoesNoWork(t *testing.T) {
	headers := http.Header{"Authorization": {"Bearer super-secret"}, "Set-Cookie": {"session=secret"}}
	body := []byte("payload")
	ctx := &gateway.Context{
		Logger: newSilentLogger(),
		Request: &gateway.Request{
			Method:     http.MethodPost,
			URL:        mustParseURL("https://example.org/test"),
			Headers:    headers,
			BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(body)), int64(len(body))),
		},
		Response: &gateway.Response{
			Status:     http.StatusOK,
			Headers:    headers,
			BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(body)), int64(len(body))),
		},
	}
	f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true)

	// Redacting copies the header map, so any work done before the level check allocates.
	allocs := testing.AllocsPerRun(100, func() {
		_ = f.PreProcess(ctx)
		_ = f.PostProcess(ctx)
	})

	if allocs != 0 {
		t.Errorf("expected %v actual %v", 0, allocs)
	}
}

func TestRequestResponseLogger_LogsBodyAsTextWhenValidUTF8(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		body     []byte
	}{
		{
			name:     "log should write the body as text when it is plain text",
			body:     []byte("hello"),
			expected: `"body":"hello"`,
		},
		{
			name:     "log should write the body as text when it is json",
			body:     []byte(`{"k1":"abc"}`),
			expected: `"body":"{\"k1\":\"abc\"}"`,
		},
		{
			name:     "log should write the body as text when it has multi-byte characters",
			body:     []byte("héllo € \U0001F600"),
			expected: "\"body\":\"héllo € \U0001F600\"",
		},
		{
			name:     "log should write the body as an empty string when there is no body",
			body:     nil,
			expected: `"body":""`,
		},
		{
			name:     "log should keep the body as bytes when it is not valid utf-8",
			body:     []byte{0xff, 0xfe, 0x00},
			expected: `"body":"//4A"`,
		},
		{
			name:     "log should keep the body as bytes when it ends with an unfinished character",
			body:     []byte("a\xc3"),
			expected: `"body":"YcM="`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true)

			t.Run("request", func(t *testing.T) {
				var buf bytes.Buffer
				ctx := newJSONLoggedRequestContext(t, &buf, nil, tt.body)

				_ = f.PreProcess(ctx)

				if !strings.Contains(buf.String(), tt.expected) {
					t.Errorf("expected: %s\nactual: %s", tt.expected, buf.String())
				}
			})
			t.Run("response", func(t *testing.T) {
				var buf bytes.Buffer
				ctx := newJSONLoggedResponseContext(t, &buf, nil, tt.body)

				_ = f.PostProcess(ctx)
				_, _ = io.Copy(io.Discard, ctx.Response.BodyReader)

				if !strings.Contains(buf.String(), tt.expected) {
					t.Errorf("expected: %s\nactual: %s", tt.expected, buf.String())
				}
			})
		})
	}
}

func TestRequestResponseLogger_PostProcess_TruncatedBodyStaysText(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		body     []byte
		limit    int64
	}{
		{
			name:     "log should drop the unfinished character when the limit cuts a two-byte character",
			body:     []byte("aébc"),
			limit:    2,
			expected: `"body":"a","bytes":5`,
		},
		{
			name:     "log should keep the character when the limit falls right after it",
			body:     []byte("aébc"),
			limit:    3,
			expected: "\"body\":\"aé\",\"bytes\":5",
		},
		{
			name:     "log should drop the unfinished character when the limit cuts a three-byte character after its first byte",
			body:     []byte("ab€d"),
			limit:    3,
			expected: `"body":"ab","bytes":6`,
		},
		{
			name:     "log should drop the unfinished character when the limit cuts a three-byte character after its second byte",
			body:     []byte("ab€d"),
			limit:    4,
			expected: `"body":"ab","bytes":6`,
		},
		{
			name:     "log should drop the unfinished character when the limit cuts a four-byte character",
			body:     []byte("\U0001F600!"),
			limit:    3,
			expected: `"body":"","bytes":5`,
		},
		{
			name:     "log should keep the character when the limit falls right after a four-byte character",
			body:     []byte("\U0001F600!"),
			limit:    4,
			expected: "\"body\":\"\U0001F600\",\"bytes\":5",
		},
		{
			name:     "log should keep the prefix as bytes when the body is not text",
			body:     []byte{0xff, 0xfe, 'a', 'b'},
			limit:    2,
			expected: `"body":"//4=","bytes":4`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			ctx := newJSONLoggedResponseContext(t, &buf, nil, tt.body)

			f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo, tt.limit, true)
			_ = f.PostProcess(ctx)
			_, _ = io.Copy(io.Discard, ctx.Response.BodyReader)

			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("expected: %s\nactual: %s", tt.expected, buf.String())
			}
		})
	}
}

func TestRequestResponseLogger_PostProcess_InterruptedStreamStaysText(t *testing.T) {
	var buf bytes.Buffer
	ctx := newJSONLoggedResponseContext(t, &buf, nil, []byte("aébc"))

	f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true)
	_ = f.PostProcess(ctx)
	// The stream stops after the first byte of the two-byte character.
	if _, err := ctx.Response.BodyReader.Read(make([]byte, 2)); err != nil {
		t.Fatalf("partial read failed: %v", err)
	}
	if err := ctx.Response.BodyReader.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	expected := `"body":"a","bytes":2`
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected: %s\nactual: %s", expected, buf.String())
	}
}

func TestNewRequestResponseLoggerFilterBuilder_InvalidLevelIsErrInvalidLogLevel(t *testing.T) {
	_, err := filter.NewRequestResponseLoggerBuilder().Build(map[string]any{"level": "warning"})

	if !errors.Is(err, filter.ErrInvalidLogLevel) {
		t.Errorf("expected err %s actual %v", filter.ErrInvalidLogLevel, err)
	}
}
