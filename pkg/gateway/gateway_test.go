package gateway_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/circuitbreaker"
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
		httpClient       gateway.HTTPClient
		expectedErr      error
		route            *gateway.Route
		request          *gateway.Request
		expectedResponse *gateway.Response
		name             string
		expectedErrMsg   string
		globalFilters    gateway.Filters
	}{
		{
			name: "Do gateway should succeed",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: &http.Response{
					StatusCode: http.StatusOK,
				},
				Err: nil,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: &url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
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
			expectedResponse: &gateway.Response{
				Status: http.StatusOK,
				Body:   nil,
			},
			expectedErr: nil,
		},
		{
			name: "Do gateway should succeed when request body is empty",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: &http.Response{
					StatusCode: http.StatusOK,
				},
				Err: nil,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: &url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
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
				Body:    []byte{},
			},
			expectedResponse: &gateway.Response{
				Status: http.StatusOK,
				Body:   nil,
			},
			expectedErr: nil,
		},
		{
			name: "Do gateway should return error when preprocess filter failed",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: &http.Response{
					StatusCode: http.StatusOK,
				},
				Err: nil,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: &url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  io.EOF,
						PostProcessErr: nil,
						ID:             "F1",
					},
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
			expectedResponse: nil,
			expectedErr:      io.EOF,
			expectedErrMsg:   "gateway request for route r1 failed: pre-process filters failed with filter F1: EOF",
		},
		{
			name: "Do gateway should return error when deadline exceeded",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      context.DeadlineExceeded,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: &url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
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
			expectedResponse: nil,
			expectedErr:      context.DeadlineExceeded,
			expectedErrMsg:   "gateway request for route r1 failed: context deadline exceeded",
		},
		{
			name: "Do gateway should return error when context cancelled",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      context.Canceled,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: &url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
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
			expectedResponse: nil,
			expectedErr:      context.DeadlineExceeded,
			expectedErrMsg:   "gateway request for route r1 failed: context deadline exceeded",
		},
		{
			name: "Do gateway should return error when generic http error",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      errors.New("someErr"),
			},
			route: &gateway.Route{
				ID: "r1",
				URI: &url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
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
			expectedResponse: nil,
			expectedErr:      gateway.ErrHTTP,
			expectedErrMsg:   "gateway request for route r1 failed: gateway http request to backend failed: someErr",
		},
		{
			name: "Do gateway should return error when generic circuit breaker is open",
			globalFilters: gateway.Filters{
				&DummyFilter{
					ID: "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      circuitbreaker.ErrOpenState,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: &url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
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
			expectedResponse: nil,
			expectedErr:      gateway.ErrCircuitBreaker,
			expectedErrMsg:   "gateway request for route r1 failed: circuit breaker failed: circuit breaker is open",
		},
		{
			name: "Do gateway should return error when generic circuit breaker is half open",
			globalFilters: gateway.Filters{
				&DummyFilter{
					ID: "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: nil,
				Err:      circuitbreaker.ErrHalfOpenRequestExceeded,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: &url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: nil,
						ID:             "F1",
					},
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
			expectedResponse: nil,
			expectedErr:      gateway.ErrCircuitBreaker,
			expectedErrMsg:   "gateway request for route r1 failed: circuit breaker failed: too many requests while circuit breaker is half-open",
		},
		{
			name: "Do gateway should return error when post process filters failed",
			globalFilters: gateway.Filters{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "GF1",
				},
			},
			httpClient: &MockHTTPClient{
				Response: &http.Response{
					StatusCode: http.StatusOK,
				},
				Err: nil,
			},
			route: &gateway.Route{
				ID: "r1",
				URI: &url.URL{
					Scheme: "https",
					Host:   "example.org",
				},
				Filters: []gateway.Filter{
					&DummyFilter{
						PreProcessErr:  nil,
						PostProcessErr: io.EOF,
						ID:             "F1",
					},
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
			expectedResponse: &gateway.Response{
				Status: http.StatusOK,
				Body:   nil,
			},
			expectedErr:    io.EOF,
			expectedErrMsg: "gateway request for route r1 failed: post-process filters failed with filter F1: EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := gateway.NewGateway(tt.globalFilters, tt.httpClient)
			ctx, _ := gateway.NewGatewayContext(tt.route, tt.request)

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
