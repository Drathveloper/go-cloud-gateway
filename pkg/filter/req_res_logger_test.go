package filter_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
			name: "build should succeed when level is present and is not valid",
			args: map[string]any{
				"level": "OTHER",
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
			if tt.body != nil {
				bodyBytes = io.NopCloser(bytes.NewBuffer(tt.body))
			}
			res := &http.Response{
				StatusCode: tt.status,
				Header:     tt.headers,
				Body:       bodyBytes,
			}
			gwRes := gateway.NewGatewayResponse(res)
			ctx, _ := gateway.NewGatewayContext(t.Context(), &gateway.Route{}, nil)
			ctx.Logger = logger
			ctx.Response = gwRes

			f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo, filter.DefaultMaxLoggedBodyBytes, true)
			_ = f.PostProcess(ctx)

			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("expected: %s\nactual: %s", tt.expected, buf.String())
			}
		})
	}
}
