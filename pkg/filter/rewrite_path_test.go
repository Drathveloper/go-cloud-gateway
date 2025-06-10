package filter_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func TestNewRewritePathBuilder(t *testing.T) {
	tests := []struct {
		expectedErr error
		args        map[string]any
		name        string
	}{
		{
			name: "build should succeed when args are present and are valid",
			args: map[string]any{
				"regexp":      "regx",
				"replacement": "repl",
			},
			expectedErr: nil,
		},
		{
			name: "build should fail when regexp argument is not valid",
			args: map[string]any{
				"replacement": "repl",
			},
			expectedErr: errors.New("failed to convert 'regexp' attribute: value is required"),
		},
		{
			name: "build should fail when replacement argument is not valid",
			args: map[string]any{
				"regexp": "regx",
			},
			expectedErr: errors.New("failed to convert 'replacement' attribute: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := filter.NewRewritePathBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestRewritePathFilter_Name(t *testing.T) {
	expected := "RewritePath"

	f, _ := filter.NewRewritePathFilter("", "")

	actual := f.Name()

	if expected != actual {
		t.Errorf("expected %s actual %s", expected, actual)
	}
}

func TestRewritePathFilter_PreProcess(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		pattern     string
		replacement string
		expected    string
	}{
		{
			name:        "rewrite should succeed when pattern matches",
			path:        "/v1/customer/person1",
			pattern:     "/v1/customer/(?<segment>.*)",
			replacement: "/api/$\\{segment}",
			expected:    "/api/person1",
		},
		{
			name:        "rewrite should do nothing when pattern doesn't match",
			path:        "/v2/customer/person1",
			pattern:     "/v1/customer/(?<segment>.*)",
			replacement: "/api/$\\{segment}",
			expected:    "/v2/customer/person1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.org"+tt.path, nil)
			gwReq, _ := gateway.NewGatewayRequest(req)
			ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, gwReq)
			f, _ := filter.NewRewritePathFilter(tt.pattern, tt.replacement)

			_ = f.PreProcess(ctx)

			if tt.expected != ctx.Request.URL.Path {
				t.Errorf("expected %s actual %s", tt.expected, ctx.Request.URL.Path)
			}
			if ctx.Attributes[filter.GatewayOriginalRequestAttr] != req.URL {
				t.Errorf("expected cached original URL %s actual %s",
					req.URL.Path, ctx.Attributes[filter.GatewayOriginalRequestAttr])
			}
		})
	}
}

func TestRewritePathFilter_PostProcess(t *testing.T) {
	f, _ := filter.NewRewritePathFilter("", "")
	if err := f.PostProcess(nil); err != nil {
		t.Errorf("expected nil err actual %s", err)
	}
}
