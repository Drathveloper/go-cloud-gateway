package gateway_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func TestNewGatewayRequest(t *testing.T) {
	tests := []struct {
		name        string
		request     *http.Request
		expected    *gateway.Request
		expectedErr error
	}{
		{
			name: "new gateway should succeed",
			request: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Header: map[string][]string{
					"h1": {"value1"},
				},
				Body: io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			},
			expected: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method: http.MethodGet,
				Headers: map[string][]string{
					"h1": {"value1"},
				},
				Body: []byte("{\"p1\":\"v1\"}"),
			},
			expectedErr: nil,
		},
		{
			name: "new gateway should succeed when body is nil",
			request: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Header: map[string][]string{
					"h1": {"value1"},
				},
				Body: nil,
			},
			expected: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method: http.MethodGet,
				Headers: map[string][]string{
					"h1": {"value1"},
				},
				Body: []byte{},
			},
			expectedErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gwReq, err := gateway.NewGatewayRequest(tt.request)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if !reflect.DeepEqual(tt.expected, gwReq) {
				t.Errorf("expected response %v actual %v", tt.expected, gwReq)
			}
		})
	}
}

func TestNewGatewayResponse(t *testing.T) {
	tests := []struct {
		name        string
		response    *http.Response
		expected    *gateway.Response
		expectedErr error
	}{
		{
			name: "new gateway should succeed",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Header: map[string][]string{
					"h1": {"value1"},
				},
				Body: io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			},
			expected: &gateway.Response{
				Status: http.StatusOK,
				Headers: map[string][]string{
					"h1": {"value1"},
				},
				Body: []byte("{\"p1\":\"v1\"}"),
			},
			expectedErr: nil,
		},
		{
			name: "new gateway should succeed when body is nil",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Header: map[string][]string{
					"h1": {"value1"},
				},
				Body: nil,
			},
			expected: &gateway.Response{
				Status: http.StatusOK,
				Headers: map[string][]string{
					"h1": {"value1"},
				},
				Body: []byte{},
			},
			expectedErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gwRes, err := gateway.NewGatewayResponse(tt.response)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if !reflect.DeepEqual(tt.expected, gwRes) {
				t.Errorf("expected response %v actual %v", tt.expected, gwRes)
			}
		})
	}
}
