package bootstrap_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
)

func TestInitialize_NotFoundHandler(t *testing.T) {
	teapot := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "no route for this request", http.StatusTeapot)
	})
	tests := []struct {
		build          func(cfg *config.Config) *bootstrap.Options
		name           string
		expectedBody   string
		expectedStatus int
	}{
		{
			name: "default handler should answer when the options builder has no not found handler",
			build: func(cfg *config.Config) *bootstrap.Options {
				return bootstrap.NewOptionsBuilder(cfg).Build()
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "route not found\n",
		},
		{
			name: "default handler should answer when the options builder is given a nil not found handler",
			build: func(cfg *config.Config) *bootstrap.Options {
				return bootstrap.NewOptionsBuilder(cfg).WithNotFoundHandler(nil).Build()
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "route not found\n",
		},
		{
			name: "default handler should answer when options built by hand have no not found handler",
			build: func(cfg *config.Config) *bootstrap.Options {
				return &bootstrap.Options{Config: cfg, GatewayErrorHandler: gatewayhandler.BaseErrorHandler()}
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "route not found\n",
		},
		{
			name: "not found handler set in the options builder should answer",
			build: func(cfg *config.Config) *bootstrap.Options {
				return bootstrap.NewOptionsBuilder(cfg).WithNotFoundHandler(teapot).Build()
			},
			expectedStatus: http.StatusTeapot,
			expectedBody:   "no route for this request\n",
		},
		{
			name: "not found handler set in options built by hand should answer",
			build: func(cfg *config.Config) *bootstrap.Options {
				return &bootstrap.Options{
					Config:              cfg,
					GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
					NotFoundHandler:     teapot,
				}
			},
			expectedStatus: http.StatusTeapot,
			expectedBody:   "no route for this request\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := readTestConfig(t, fmt.Sprintf(
				`{"gateway":{"routes":[{"id":"r1","uri":%q,"predicates":[{"name":"Path","args":{"patterns":["/known"]}}]}]}}`,
				newStatusBackend(t, http.StatusNoContent).URL))
			gatewayServer := startTestGatewayWithOptions(t, tt.build(cfg))

			status, body, err := getTestURL(t, gatewayServer.Client(), gatewayServer.URL+"/unknown")
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}
			// The not found handler must not take over the requests that do match a route.
			knownStatus, _, err := getTestURL(t, gatewayServer.Client(), gatewayServer.URL+"/known")
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}

			if status != tt.expectedStatus {
				t.Errorf("expected status %d actual %d", tt.expectedStatus, status)
			}
			if string(body) != tt.expectedBody {
				t.Errorf("expected body %q actual %q", tt.expectedBody, body)
			}
			if knownStatus != http.StatusNoContent {
				t.Errorf("expected status %d for a matched route actual %d", http.StatusNoContent, knownStatus)
			}
		})
	}
}
