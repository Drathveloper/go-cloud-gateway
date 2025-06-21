package gatewayhandler_test

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

type mockGateway struct {
	doFunc func(ctx *gateway.Context) error
}

func (m *mockGateway) Do(ctx *gateway.Context) error {
	return m.doFunc(ctx)
}

type mockErrorHandler struct {
	handleFunc func(logger *slog.Logger, err error, w http.ResponseWriter)
}

func (m *mockErrorHandler) Handle(logger *slog.Logger, err error, w http.ResponseWriter) {
	m.handleFunc(logger, err, w)
}

func TestGatewayHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		gatewayErr       error
		expectedErr      error
		request          *http.Request
		expectedResponse gateway.Response
		name             string
		expectedBody     string
		routes           gateway.Routes
	}{
		{
			name: "serve HTTP should succeed when request is valid and route is found",
			routes: gateway.Routes{
				{
					ID: "r1",
					Predicates: gateway.Predicates{
						predicate.NewMethodPredicate(http.MethodGet),
					},
				},
			},
			request: httptest.NewRequest(http.MethodGet, "http://localhost:8080/test", nil),
			expectedResponse: gateway.Response{
				Headers: map[string][]string{
					"Content-Length": {"4"},
				},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBufferString("test")), int64(4)),
				Status:     http.StatusOK,
			},
			expectedBody: "test",
			gatewayErr:   nil,
		},
		{
			name: "serve HTTP should succeed when request is valid and response doesn't have content length",
			routes: gateway.Routes{
				{
					ID: "r1",
					Predicates: gateway.Predicates{
						predicate.NewMethodPredicate(http.MethodGet),
					},
				},
			},
			request: httptest.NewRequest(http.MethodGet, "http://localhost:8080/test", nil),
			expectedResponse: gateway.Response{
				Headers: map[string][]string{
					"Transfer-Encoding": {"chunked"},
				},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBufferString("test")), int64(-1)),
				Status:     http.StatusOK,
			},
			expectedBody: "test",
			gatewayErr:   nil,
		},
		{
			name: "serve HTTP should handle error when request is valid and route isn't found",
			routes: gateway.Routes{
				{
					ID: "r1",
					Predicates: gateway.Predicates{
						predicate.NewMethodPredicate(http.MethodPost),
					},
				},
			},
			request:     httptest.NewRequest(http.MethodGet, "http://localhost:8080/test", nil),
			expectedErr: gatewayhandler.ErrRouteNotFound,
		},
		{
			name: "serve HTTP should handle error when gateway request failed",
			routes: gateway.Routes{
				{
					ID: "r1",
					Predicates: gateway.Predicates{
						predicate.NewMethodPredicate(http.MethodGet),
					},
				},
			},
			request:     httptest.NewRequest(http.MethodGet, "http://localhost:8080/test", nil),
			gatewayErr:  gateway.ErrHTTP,
			expectedErr: gateway.ErrHTTP,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			gw := &mockGateway{
				doFunc: func(ctx *gateway.Context) error {
					ctx.Response = &tt.expectedResponse
					return tt.gatewayErr
				},
			}
			errHandler := &mockErrorHandler{
				handleFunc: func(_ *slog.Logger, err error, _ http.ResponseWriter) {
					if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
						t.Errorf("expected err %s actual %s", tt.expectedErr, err)
					}
				},
			}
			gwHandler := gatewayhandler.NewGatewayHandler(gw, tt.routes, errHandler)

			gwHandler.ServeHTTP(recorder, tt.request)

			if tt.expectedErr == nil && recorder.Code != tt.expectedResponse.Status {
				t.Errorf("expected status %d actual %d", tt.expectedResponse.Status, recorder.Code)
			}
			if tt.expectedErr == nil && recorder.Body.String() != tt.expectedBody {
				t.Errorf("expected body %s actual %s", tt.expectedBody, recorder.Body.String())
			}
			if tt.expectedErr == nil && !reflect.DeepEqual(recorder.Header(), tt.expectedResponse.Headers) {
				t.Errorf("expected headers %v actual %v", tt.expectedResponse.Headers, recorder.Header())
			}
		})
	}
}
