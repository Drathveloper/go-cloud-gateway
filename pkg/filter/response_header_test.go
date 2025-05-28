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

func TestNewAddResponseHeaderBuilder(t *testing.T) {
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
			actual, err := filter.NewAddResponseHeaderBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestAddResponseHeaderFilter_Name(t *testing.T) {
	expected := "AddResponseHeader"

	f := filter.NewAddResponseHeaderFilter("", "")

	actual := f.Name()

	if expected != actual {
		t.Errorf("expected %s actual %s", expected, actual)
	}
}

func TestAddResponseHeaderFilter_PostProcess(t *testing.T) {
	tests := []struct {
		name            string
		headerKey       string
		headerValue     string
		currentHeaders  http.Header
		expectedHeaders http.Header
	}{
		{
			name:            "add response header should succeed when header not present",
			headerKey:       "X-Test-Header",
			headerValue:     "True",
			currentHeaders:  map[string][]string{"Accept-Language": {"es_ES"}},
			expectedHeaders: map[string][]string{"Accept-Language": {"es_ES"}, "X-Test-Header": {"True"}},
		},
		{
			name:            "add response header should succeed when header already present",
			headerKey:       "X-Test-Header",
			headerValue:     "True",
			currentHeaders:  map[string][]string{"Accept-Language": {"es_ES"}, "X-Test-Header": {"False"}},
			expectedHeaders: map[string][]string{"Accept-Language": {"es_ES"}, "X-Test-Header": {"False", "True"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "/", nil)
			gwReq, _ := gateway.NewGatewayRequest(req)
			ctx, _ := gateway.NewGatewayContext(nil, gwReq, nil, 0)
			res := &http.Response{
				StatusCode: http.StatusOK,
				Header:     tt.currentHeaders,
			}
			gwRes, _ := gateway.NewGatewayResponse(res)
			ctx.Response = gwRes

			f := filter.NewAddResponseHeaderFilter(tt.headerKey, tt.headerValue)

			_ = f.PostProcess(ctx)

			for k, valueList := range tt.expectedHeaders {
				if !slices.Equal(valueList, ctx.Response.Headers[k]) {
					t.Errorf("expected %v actual %v", valueList, ctx.Request.Headers[k])
				}
			}
		})
	}
}

func TestAddResponseHeaderFilter_PreProcess(t *testing.T) {
	f := filter.NewAddResponseHeaderFilter("", "")
	if err := f.PreProcess(nil); err != nil {
		t.Errorf("expected nil err actual %s", err)
	}
}

func TestNewSetResponseHeaderBuilder(t *testing.T) {
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
			actual, err := filter.NewSetResponseHeaderBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestSetResponseHeaderFilter_Name(t *testing.T) {
	expected := "SetResponseHeader"

	f := filter.NewSetResponseHeaderFilter("", "")

	actual := f.Name()

	if expected != actual {
		t.Errorf("expected %s actual %s", expected, actual)
	}
}

func TestSetResponseHeaderFilter_PostProcess(t *testing.T) {
	tests := []struct {
		name            string
		headerKey       string
		headerValue     string
		currentHeaders  http.Header
		expectedHeaders http.Header
	}{
		{
			name:            "set response header should succeed when header not present",
			headerKey:       "X-Test-Header",
			headerValue:     "True",
			currentHeaders:  map[string][]string{"Accept-Language": {"es_ES"}},
			expectedHeaders: map[string][]string{"Accept-Language": {"es_ES"}, "X-Test-Header": {"True"}},
		},
		{
			name:            "set response header should succeed when header already present",
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
			ctx, _ := gateway.NewGatewayContext(nil, gwReq, nil, 0)
			res := &http.Response{
				StatusCode: http.StatusOK,
				Header:     tt.currentHeaders,
			}
			gwRes, _ := gateway.NewGatewayResponse(res)
			ctx.Response = gwRes

			f := filter.NewSetResponseHeaderFilter(tt.headerKey, tt.headerValue)

			_ = f.PostProcess(ctx)

			for k, valueList := range tt.expectedHeaders {
				if !slices.Equal(valueList, ctx.Response.Headers[k]) {
					t.Errorf("expected %v actual %v", valueList, ctx.Request.Headers[k])
				}
			}
		})
	}
}

func TestSetResponseHeaderFilter_PreProcess(t *testing.T) {
	f := filter.NewSetResponseHeaderFilter("", "")
	if err := f.PreProcess(nil); err != nil {
		t.Errorf("expected nil err actual %s", err)
	}
}

func TestNewRemoveResponseHeaderBuilder(t *testing.T) {
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
			actual, err := filter.NewRemoveResponseHeaderBuilder().Build(tt.args)

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err == nil && actual == nil {
				t.Errorf("expected %v to be present", actual)
			}
		})
	}
}

func TestRemoveResponseHeaderFilter_Name(t *testing.T) {
	expected := "RemoveResponseHeader"

	f := filter.NewRemoveResponseHeaderFilter("")

	actual := f.Name()

	if expected != actual {
		t.Errorf("expected %s actual %s", expected, actual)
	}
}

func TestRemoveResponseHeaderFilter_PostProcess(t *testing.T) {
	tests := []struct {
		name            string
		headerKey       string
		currentHeaders  http.Header
		expectedHeaders http.Header
	}{
		{
			name:            "remove response header should succeed when header not present",
			headerKey:       "X-Test-Header",
			currentHeaders:  map[string][]string{"Accept-Language": {"es_ES"}},
			expectedHeaders: map[string][]string{"Accept-Language": {"es_ES"}},
		},
		{
			name:            "remove response header should succeed when header already present",
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
			ctx, _ := gateway.NewGatewayContext(nil, gwReq, nil, 0)
			res := &http.Response{
				StatusCode: http.StatusOK,
				Header:     tt.currentHeaders,
			}
			gwRes, _ := gateway.NewGatewayResponse(res)
			ctx.Response = gwRes

			f := filter.NewRemoveResponseHeaderFilter(tt.headerKey)

			_ = f.PostProcess(ctx)

			for k, valueList := range tt.expectedHeaders {
				if !slices.Equal(valueList, ctx.Response.Headers[k]) {
					t.Errorf("expected %v actual %v", valueList, ctx.Request.Headers[k])
				}
			}
		})
	}
}

func TestRemoveResponseHeaderFilter_PreProcess(t *testing.T) {
	f := filter.NewRemoveResponseHeaderFilter("")
	if err := f.PreProcess(nil); err != nil {
		t.Errorf("expected nil err actual %s", err)
	}
}
