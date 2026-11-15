package bootstrap_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
)

// newCodeConfig returns a config built in code, as an application that reads no config
// file does, with one route to the given backend and a circuit breaker.
func newCodeConfig(backendURL string) *config.Config {
	return &config.Config{
		Gateway: config.Gateway{
			Routes: []config.Route{
				{
					ID:  "r1",
					URI: backendURL,
					CircuitBreaker: config.CircuitBreaker{
						Enabled:                 true,
						Interval:                config.Duration{Duration: time.Minute},
						FailureRateThreshold:    50,
						NumAllowedHalfOpenCalls: 1,
						WaitDurationInOpenState: config.Duration{Duration: time.Minute},
						MinRequestsThreshold:    2,
					},
				},
			},
		},
	}
}

func TestInitialize_ValidConfigBuiltInCodeServesRequests(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("backend"))
	}))
	t.Cleanup(backend.Close)
	enabled := false
	responseHeaderTimeout := config.Duration{}
	cfg := newCodeConfig(backend.URL)
	cfg.Gateway.HTTPClient = &config.HTTPClient{
		MTLS: &config.MTLS{Enabled: &enabled},
		Pool: &config.Pool{
			Timeout:             &config.Duration{Duration: time.Second},
			KeepAlive:           &config.Duration{Duration: time.Second},
			IdleConnTimeout:     &config.Duration{Duration: time.Second},
			TLSHandshakeTimeout: &config.Duration{Duration: time.Second},
			// A pool duration has to be set, but it can be set to zero.
			ResponseHeaderTimeout: &responseHeaderTimeout,
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   10,
			MaxConnsPerHost:       10,
		},
	}
	gatewayServer := startTestGatewayWithOptions(t, bootstrap.NewOptionsBuilder(cfg).Build())

	status, body, err := getTestURL(t, gatewayServer.Client(), gatewayServer.URL)

	if err != nil {
		t.Fatalf("expected nil err actual %v", err)
	}
	if status != http.StatusOK || string(body) != "backend" {
		t.Errorf("expected 200 backend actual %d %s", status, body)
	}
}

// A config built in code skips the readers, which were the only ones that validated: it
// reached the gateway whatever it held. Initialize now applies the same rules.
func TestInitialize_InvalidConfigBuiltInCode(t *testing.T) {
	const backendURL = "http://localhost:8001"
	breaker := func(change func(breaker *config.CircuitBreaker)) *config.Config {
		cfg := newCodeConfig(backendURL)
		change(&cfg.Gateway.Routes[0].CircuitBreaker)
		return cfg
	}
	tests := []struct {
		cfg      *config.Config
		name     string
		expected string
	}{
		{
			name:     "initialize should fail when the config is nil",
			cfg:      nil,
			expected: "config is nil",
		},
		{
			name:     "initialize should fail when the config has no routes",
			cfg:      &config.Config{},
			expected: "Key: 'Config.Gateway.Routes' Error:Field validation for 'Routes' failed on the 'required' tag",
		},
		{
			name: "initialize should fail when a route has no id",
			cfg: &config.Config{
				Gateway: config.Gateway{Routes: []config.Route{{URI: backendURL}}},
			},
			expected: "Key: 'Config.Gateway.Routes[0].ID' Error:Field validation for 'ID' failed on the 'required' tag",
		},
		{
			name: "initialize should fail when mtls does not say whether it is enabled",
			cfg: &config.Config{
				Gateway: config.Gateway{
					HTTPClient: &config.HTTPClient{MTLS: &config.MTLS{}},
					Routes:     []config.Route{{ID: "r1", URI: backendURL}},
				},
			},
			expected: "Key: 'Config.Gateway.HTTPClient.MTLS.Enabled' Error:Field validation for 'Enabled' failed on the 'required' tag",
		},
		{
			name: "initialize should fail when failure rate threshold is over 100",
			cfg: breaker(func(breaker *config.CircuitBreaker) {
				breaker.FailureRateThreshold = 101
			}),
			expected: "Key: 'Config.Gateway.Routes[0].CircuitBreaker.FailureRateThreshold' " +
				"Error:Field validation for 'FailureRateThreshold' failed on the 'max' tag",
		},
		{
			name: "initialize should fail when min requests threshold is negative",
			cfg: breaker(func(breaker *config.CircuitBreaker) {
				breaker.MinRequestsThreshold = -1
			}),
			expected: "Key: 'Config.Gateway.Routes[0].CircuitBreaker.MinRequestsThreshold' " +
				"Error:Field validation for 'MinRequestsThreshold' failed on the 'min' tag",
		},
		{
			name: "initialize should fail when num allowed half open calls is negative",
			cfg: breaker(func(breaker *config.CircuitBreaker) {
				breaker.NumAllowedHalfOpenCalls = -1
			}),
			expected: "Key: 'Config.Gateway.Routes[0].CircuitBreaker.NumAllowedHalfOpenCalls' " +
				"Error:Field validation for 'NumAllowedHalfOpenCalls' failed on the 'min' tag",
		},
		{
			name: "initialize should fail when wait duration in open state is negative",
			cfg: breaker(func(breaker *config.CircuitBreaker) {
				breaker.WaitDurationInOpenState = config.Duration{Duration: -time.Minute}
			}),
			expected: "duration must not be negative: " +
				"'Config.Gateway.Routes[0].CircuitBreaker.WaitDurationInOpenState' is -1m0s",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := bootstrap.Initialize(bootstrap.NewOptionsBuilder(tt.cfg).Build())

			if expected := "gateway initialization failed: validate config failed: " + tt.expected; err == nil ||
				err.Error() != expected {
				t.Errorf("expected %s actual %v", expected, err)
			}
			if server != nil {
				t.Errorf("expected nil server actual %v", server)
			}
		})
	}
}

func TestInitialize_InvalidConfigErrorsCanBeInspected(t *testing.T) {
	t.Run("nil config should be config.ErrNilConfig", func(t *testing.T) {
		_, err := bootstrap.Initialize(bootstrap.NewOptionsBuilder(nil).Build())

		if !errors.Is(err, config.ErrNilConfig) {
			t.Errorf("expected %v actual %v", config.ErrNilConfig, err)
		}
	})
	t.Run("broken tags should be validator.ValidationErrors", func(t *testing.T) {
		_, err := bootstrap.Initialize(bootstrap.NewOptionsBuilder(&config.Config{}).Build())

		var validationErrs validator.ValidationErrors
		if !errors.As(err, &validationErrs) {
			t.Fatalf("expected validator.ValidationErrors actual %v", err)
		}
		if len(validationErrs) != 1 || !strings.HasSuffix(validationErrs[0].Namespace(), "Gateway.Routes") {
			t.Errorf("expected one error for the routes actual %v", validationErrs)
		}
	})
}
