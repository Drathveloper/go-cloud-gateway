package filter_test

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func TestNewAddRequestHeaderBuilder(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		expectedErr error
	}{
		{
			name: "build should succeed when args are present and are valid",
			args: map[string]any{
				"name":  "First",
				"value": "any1",
			},
			expectedErr: nil,
		},
		{
			name: "build should fail when name argument is not valid",
			args: map[string]any{
				"value": "any1",
			},
			expectedErr: errors.New("failed to convert 'name' attribute: value is required"),
		},
		{
			name: "build should fail when value argument is not valid",
			args: map[string]any{
				"name": "First",
			},
			expectedErr: errors.New("failed to convert 'value' attribute: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := filter.NewAddRequestHeaderBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestAddRequestHeaderFilter_Name(t *testing.T) {
	expected := "AddRequestHeader"

	f := filter.NewAddRequestHeaderFilter("", "")

	actual := f.Name()

	if expected != actual {
		t.Errorf("expected %s actual %s", expected, actual)
	}
}

func TestAddRequestHeaderFilter_PreProcess(t *testing.T) {
	tests := []struct {
		name            string
		headerKey       string
		headerValue     string
		currentHeaders  http.Header
		expectedHeaders http.Header
	}{
		{
			name:            "add request header should succeed when header not present",
			headerKey:       "X-Test-Header",
			headerValue:     "True",
			currentHeaders:  map[string][]string{"Accept-Language": {"es_ES"}},
			expectedHeaders: map[string][]string{"Accept-Language": {"es_ES"}, "X-Test-Header": {"True"}},
		},
		{
			name:            "add request header should succeed when header already present",
			headerKey:       "X-Test-Header",
			headerValue:     "True",
			currentHeaders:  map[string][]string{"Accept-Language": {"es_ES"}, "X-Test-Header": {"False"}},
			expectedHeaders: map[string][]string{"Accept-Language": {"es_ES"}, "X-Test-Header": {"False", "True"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "/", nil)
			req.Header = tt.currentHeaders
			gwReq, _ := gateway.NewGatewayRequest(req)
			ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, gwReq, nil)

			f := filter.NewAddRequestHeaderFilter(tt.headerKey, tt.headerValue)

			_ = f.PreProcess(ctx)

			for k, valueList := range tt.expectedHeaders {
				if !slices.Equal(valueList, ctx.Request.Headers[k]) {
					t.Errorf("expected %v actual %v", valueList, ctx.Request.Headers[k])
				}
			}
		})
	}
}

func TestAddRequestHeaderFilter_PostProcess(t *testing.T) {
	f := filter.NewAddRequestHeaderFilter("", "")
	if err := f.PostProcess(nil); err != nil {
		t.Errorf("expected nil err actual %s", err)
	}
}

func TestNewSetRequestHeaderBuilder(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		expectedErr error
	}{
		{
			name: "build should succeed when args are present and are valid",
			args: map[string]any{
				"name":  "First",
				"value": "any1",
			},
			expectedErr: nil,
		},
		{
			name: "build should fail when name argument is not valid",
			args: map[string]any{
				"value": "any1",
			},
			expectedErr: errors.New("failed to convert 'name' attribute: value is required"),
		},
		{
			name: "build should fail when value argument is not valid",
			args: map[string]any{
				"name": "First",
			},
			expectedErr: errors.New("failed to convert 'value' attribute: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := filter.NewSetRequestHeaderBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestSetRequestHeaderFilter_Name(t *testing.T) {
	expected := "SetRequestHeader"

	f := filter.NewSetRequestHeaderFilter("", "")

	actual := f.Name()

	if expected != actual {
		t.Errorf("expected %s actual %s", expected, actual)
	}
}

func TestSetRequestHeaderFilter_PreProcess(t *testing.T) {
	tests := []struct {
		name            string
		headerKey       string
		headerValue     string
		currentHeaders  http.Header
		expectedHeaders http.Header
	}{
		{
			name:            "set request header should succeed when header not present",
			headerKey:       "X-Test-Header",
			headerValue:     "True",
			currentHeaders:  map[string][]string{"Accept-Language": {"es_ES"}},
			expectedHeaders: map[string][]string{"Accept-Language": {"es_ES"}, "X-Test-Header": {"True"}},
		},
		{
			name:            "set request header should succeed when header already present",
			headerKey:       "X-Test-Header",
			headerValue:     "True",
			currentHeaders:  map[string][]string{"Accept-Language": {"es_ES"}, "X-Test-Header": {"False"}},
			expectedHeaders: map[string][]string{"Accept-Language": {"es_ES"}, "X-Test-Header": {"True"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "/", nil)
			req.Header = tt.currentHeaders
			gwReq, _ := gateway.NewGatewayRequest(req)
			ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, gwReq, nil)

			f := filter.NewSetRequestHeaderFilter(tt.headerKey, tt.headerValue)

			_ = f.PreProcess(ctx)

			for k, valueList := range tt.expectedHeaders {
				if !slices.Equal(valueList, ctx.Request.Headers[k]) {
					t.Errorf("expected %v actual %v", valueList, ctx.Request.Headers[k])
				}
			}
		})
	}
}

func TestSetRequestHeaderFilter_PostProcess(t *testing.T) {
	f := filter.NewSetRequestHeaderFilter("", "")
	if err := f.PostProcess(nil); err != nil {
		t.Errorf("expected nil err actual %s", err)
	}
}

func TestNewRemoveRequestHeaderBuilder(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		expectedErr error
	}{
		{
			name: "build should succeed when args are present and are valid",
			args: map[string]any{
				"name": "First",
			},
			expectedErr: nil,
		},
		{
			name:        "build should fail when name argument is not valid",
			args:        map[string]any{},
			expectedErr: errors.New("failed to convert 'name' attribute: value is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := filter.NewRemoveRequestHeaderBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestRemoveRequestHeaderFilter_Name(t *testing.T) {
	expected := "RemoveRequestHeader"

	f := filter.NewRemoveRequestHeaderFilter("")

	actual := f.Name()

	if expected != actual {
		t.Errorf("expected %s actual %s", expected, actual)
	}
}

func TestRemoveRequestHeaderFilter_PreProcess(t *testing.T) {
	tests := []struct {
		name            string
		headerKey       string
		currentHeaders  http.Header
		expectedHeaders http.Header
	}{
		{
			name:            "remove request header should succeed when header not present",
			headerKey:       "X-Test-Header",
			currentHeaders:  map[string][]string{"Accept-Language": {"es_ES"}},
			expectedHeaders: map[string][]string{"Accept-Language": {"es_ES"}},
		},
		{
			name:            "remove request header should succeed when header already present",
			headerKey:       "X-Test-Header",
			currentHeaders:  map[string][]string{"Accept-Language": {"es_ES"}, "X-Test-Header": {"False"}},
			expectedHeaders: map[string][]string{"Accept-Language": {"es_ES"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "/", nil)
			req.Header = tt.currentHeaders
			gwReq, _ := gateway.NewGatewayRequest(req)
			ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, gwReq, nil)

			f := filter.NewRemoveRequestHeaderFilter(tt.headerKey)

			_ = f.PreProcess(ctx)

			for k, valueList := range tt.expectedHeaders {
				if !slices.Equal(valueList, ctx.Request.Headers[k]) {
					t.Errorf("expected %v actual %v", valueList, ctx.Request.Headers[k])
				}
			}
		})
	}
}

func TestRemoveRequestHeaderFilter_PostProcess(t *testing.T) {
	f := filter.NewRemoveRequestHeaderFilter("")
	if err := f.PostProcess(nil); err != nil {
		t.Errorf("expected nil err actual %s", err)
	}
}
