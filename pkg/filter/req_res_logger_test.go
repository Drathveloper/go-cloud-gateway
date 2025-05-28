package filter_test

import (
	"bytes"
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
		name        string
		args        map[string]any
		expectedErr error
	}{
		{
			name:        "build should succeed when no args present",
			args:        map[string]any{},
			expectedErr: nil,
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

	f := filter.NewRequestResponseLoggerFilter()

	actual := f.Name()

	if expected != actual {
		t.Errorf("expected %s actual %s", expected, actual)
	}
}

func TestRequestResponseLogger_PreProcess(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		method   string
		headers  http.Header
		body     []byte
		expected string
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
			req, _ := http.NewRequest(tt.method, "https://example.org/"+tt.path, bytes.NewBuffer(tt.body))
			req.Header = tt.headers
			gwReq, _ := gateway.NewGatewayRequest(req)
			ctx, _ := gateway.NewGatewayContext(nil, gwReq, logger, 0)

			f := filter.NewRequestResponseLoggerFilter()
			_ = f.PreProcess(ctx)

			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("expected: %s\nactual: %s", tt.expected, buf.String())
			}
		})
	}
}

func TestRequestResponseLogger_PostProcess(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		headers  http.Header
		body     []byte
		expected string
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
			gwRes, _ := gateway.NewGatewayResponse(res)
			ctx, _ := gateway.NewGatewayContext(nil, nil, logger, 0)
			ctx.Response = gwRes

			f := filter.NewRequestResponseLoggerFilter()
			_ = f.PostProcess(ctx)

			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("expected: %s\nactual: %s", tt.expected, buf.String())
			}
		})
	}
}
