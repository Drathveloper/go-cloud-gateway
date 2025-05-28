package gateway_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

type MockHTTPClient struct {
	Response *http.Response
	Err      error
}

func (c *MockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	return c.Response, c.Err
}

func TestGateway_Do(t *testing.T) {
	tests := []struct {
		name             string
		globalFilters    gateway.Filters
		httpClient       gateway.HTTPClient
		route            *gateway.Route
		request          *gateway.Request
		logger           *slog.Logger
		expectedResponse *gateway.Response
		expectedErr      error
		expectedErrMsg   string
	}{
		{
			name: "Do gateway should succeed",
			globalFilters: gateway.Filters{
				&DummyFilter{"GF1", nil, nil},
			},
			httpClient: &MockHTTPClient{
				Response: &http.Response{
					StatusCode: http.StatusOK,
				},
				Err: nil,
			},
			route: &gateway.Route{
				ID:  "r1",
				URI: "/test",
				Filters: []gateway.Filter{
					&DummyFilter{"F1", nil, nil},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:  http.MethodPost,
				Headers: http.Header{},
				Body:    []byte("someBody"),
			},
			logger: slog.Default(),
			expectedResponse: &gateway.Response{
				Status: http.StatusOK,
				Body:   []byte{},
			},
			expectedErr: nil,
		},
		{
			name: "Do gateway should return error when preprocess filter failed",
			globalFilters: gateway.Filters{
				&DummyFilter{"GF1", nil, nil},
			},
			httpClient: &MockHTTPClient{
				Response: &http.Response{
					StatusCode: http.StatusOK,
				},
				Err: nil,
			},
			route: &gateway.Route{
				ID:  "r1",
				URI: "/test",
				Filters: []gateway.Filter{
					&DummyFilter{"F1", io.EOF, nil},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:  http.MethodPost,
				Headers: http.Header{},
				Body:    []byte("someBody"),
			},
			logger:           slog.Default(),
			expectedResponse: nil,
			expectedErr:      io.EOF,
			expectedErrMsg:   "gateway request for route r1 failed: pre-process filters failed with filter F1: EOF",
		},
		{
			name: "Do gateway should return error when deadline exceeded",
			globalFilters: gateway.Filters{
				&DummyFilter{"GF1", nil, nil},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      context.DeadlineExceeded,
			},
			route: &gateway.Route{
				ID:  "r1",
				URI: "/test",
				Filters: []gateway.Filter{
					&DummyFilter{"F1", nil, nil},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:  http.MethodPost,
				Headers: http.Header{},
				Body:    []byte("someBody"),
			},
			logger:           slog.Default(),
			expectedResponse: nil,
			expectedErr:      context.DeadlineExceeded,
			expectedErrMsg:   "gateway request for route r1 failed: context deadline exceeded",
		},
		{
			name: "Do gateway should return error when context cancelled",
			globalFilters: gateway.Filters{
				&DummyFilter{"GF1", nil, nil},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      context.Canceled,
			},
			route: &gateway.Route{
				ID:  "r1",
				URI: "/test",
				Filters: []gateway.Filter{
					&DummyFilter{"F1", nil, nil},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:  http.MethodPost,
				Headers: http.Header{},
				Body:    []byte("someBody"),
			},
			logger:           slog.Default(),
			expectedResponse: nil,
			expectedErr:      context.DeadlineExceeded,
			expectedErrMsg:   "gateway request for route r1 failed: context deadline exceeded",
		},
		{
			name: "Do gateway should return error when generic http error",
			globalFilters: gateway.Filters{
				&DummyFilter{"GF1", nil, nil},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      errors.New("someErr"),
			},
			route: &gateway.Route{
				ID:  "r1",
				URI: "/test",
				Filters: []gateway.Filter{
					&DummyFilter{"F1", nil, nil},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:  http.MethodPost,
				Headers: http.Header{},
				Body:    []byte("someBody"),
			},
			logger:           slog.Default(),
			expectedResponse: nil,
			expectedErr:      gateway.ErrHTTP,
			expectedErrMsg:   "gateway request for route r1 failed: gateway http request to backend failed: someErr",
		},
		{
			name: "Do gateway should return error when post process filters failed",
			globalFilters: gateway.Filters{
				&DummyFilter{"GF1", nil, nil},
			},
			httpClient: &MockHTTPClient{
				Response: &http.Response{
					StatusCode: http.StatusOK,
				},
				Err: nil,
			},
			route: &gateway.Route{
				ID:  "r1",
				URI: "/test",
				Filters: []gateway.Filter{
					&DummyFilter{"F1", nil, io.EOF},
				},
			},
			request: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method:  http.MethodPost,
				Headers: http.Header{},
				Body:    []byte("someBody"),
			},
			logger: slog.Default(),
			expectedResponse: &gateway.Response{
				Status: http.StatusOK,
				Body:   []byte{},
			},
			expectedErr:    io.EOF,
			expectedErrMsg: "gateway request for route r1 failed: post-process filters failed with filter F1: EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := gateway.NewGateway(tt.globalFilters, tt.httpClient)
			ctx, _ := gateway.NewGatewayContext(tt.route, tt.request, tt.logger)

			err := gw.Do(ctx)

			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if err != nil && tt.expectedErrMsg != err.Error() {
				t.Errorf("expected err msg %s actual %s", tt.expectedErrMsg, err)
			}
			if !reflect.DeepEqual(tt.expectedResponse, ctx.Response) {
				t.Errorf("expected response %v actual %v", tt.expectedResponse, ctx.Response)
			}
		})
	}
}
