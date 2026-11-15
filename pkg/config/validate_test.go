package config_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
)

// newValidConfig returns a config with one route, changed by the given function if any.
func newValidConfig(change func(cfg *config.Config)) *config.Config {
	cfg := &config.Config{
		Gateway: config.Gateway{
			Routes: []config.Route{{ID: "r1", URI: "http://localhost:8001"}},
		},
	}
	if change != nil {
		change(cfg)
	}
	return cfg
}

// newCircuitBreakerConfig returns a valid config whose route has an enabled circuit
// breaker, changed by the given function if any.
func newCircuitBreakerConfig(change func(breaker *config.CircuitBreaker)) *config.Config {
	return newValidConfig(func(cfg *config.Config) {
		breaker := &cfg.Gateway.Routes[0].CircuitBreaker
		*breaker = config.CircuitBreaker{
			Enabled:                 true,
			Interval:                config.Duration{Duration: time.Minute},
			FailureRateThreshold:    50,
			NumAllowedHalfOpenCalls: 5,
			WaitDurationInOpenState: config.Duration{Duration: 30 * time.Second},
			MinRequestsThreshold:    20,
		}
		if change != nil {
			change(breaker)
		}
	})
}

func TestValidate(t *testing.T) {
	enabled := true
	tests := []struct {
		cfg  *config.Config
		name string
		// expected holds a part of the message of each expected error, none when the
		// config is valid.
		expected []string
	}{
		{
			name: "validate should succeed when the config has a route",
			cfg:  newValidConfig(nil),
		},
		{
			name: "validate should succeed when the circuit breaker is enabled and configured",
			cfg:  newCircuitBreakerConfig(nil),
		},
		{
			name: "validate should succeed when the circuit breaker counters are the lowest ones",
			cfg: newCircuitBreakerConfig(func(breaker *config.CircuitBreaker) {
				breaker.MinRequestsThreshold = 1
				breaker.NumAllowedHalfOpenCalls = 1
			}),
		},
		{
			name: "validate should succeed when the circuit breaker is disabled and not configured",
			cfg: newCircuitBreakerConfig(func(breaker *config.CircuitBreaker) {
				*breaker = config.CircuitBreaker{}
			}),
		},
		{
			name:     "validate should fail when the config is nil",
			cfg:      nil,
			expected: []string{"config is nil"},
		},
		{
			name:     "validate should fail when the config has no routes",
			cfg:      &config.Config{},
			expected: []string{"'Config.Gateway.Routes' Error:Field validation for 'Routes' failed on the 'required' tag"},
		},
		{
			name: "validate should fail when a route has no id",
			cfg: newValidConfig(func(cfg *config.Config) {
				cfg.Gateway.Routes[0].ID = ""
			}),
			expected: []string{"'Config.Gateway.Routes[0].ID' Error:Field validation for 'ID' failed on the 'required' tag"},
		},
		{
			name: "validate should fail when mtls does not say whether it is enabled",
			cfg: newValidConfig(func(cfg *config.Config) {
				cfg.Gateway.HTTPClient = &config.HTTPClient{MTLS: &config.MTLS{}}
			}),
			expected: []string{"'Config.Gateway.HTTPClient.MTLS.Enabled' Error:Field validation for 'Enabled' failed on the 'required' tag"},
		},
		{
			name: "validate should fail when mtls is enabled without certificates",
			cfg: newValidConfig(func(cfg *config.Config) {
				cfg.Gateway.HTTPClient = &config.HTTPClient{MTLS: &config.MTLS{Enabled: &enabled}}
			}),
			expected: []string{"'CA' failed on the 'required_if' tag", "'Cert' failed on the 'required_if' tag", "'Key' failed on the 'required_if' tag"},
		},
		{
			name: "validate should fail when the circuit breaker is enabled and min requests threshold is zero",
			cfg: newCircuitBreakerConfig(func(breaker *config.CircuitBreaker) {
				breaker.MinRequestsThreshold = 0
			}),
			expected: []string{"'MinRequestsThreshold' failed on the 'required_if' tag"},
		},
		{
			name: "validate should fail when min requests threshold is negative",
			cfg: newCircuitBreakerConfig(func(breaker *config.CircuitBreaker) {
				breaker.MinRequestsThreshold = -1
			}),
			expected: []string{"'MinRequestsThreshold' failed on the 'min' tag"},
		},
		{
			name: "validate should fail when the circuit breaker is enabled and num allowed half open calls is zero",
			cfg: newCircuitBreakerConfig(func(breaker *config.CircuitBreaker) {
				breaker.NumAllowedHalfOpenCalls = 0
			}),
			expected: []string{"'NumAllowedHalfOpenCalls' failed on the 'required_if' tag"},
		},
		{
			name: "validate should fail when num allowed half open calls is negative",
			cfg: newCircuitBreakerConfig(func(breaker *config.CircuitBreaker) {
				breaker.NumAllowedHalfOpenCalls = -1
			}),
			expected: []string{"'NumAllowedHalfOpenCalls' failed on the 'min' tag"},
		},
		{
			name: "validate should fail when interval is negative",
			cfg: newCircuitBreakerConfig(func(breaker *config.CircuitBreaker) {
				breaker.Interval = config.Duration{Duration: -time.Minute}
			}),
			expected: []string{"duration must not be negative: 'Config.Gateway.Routes[0].CircuitBreaker.Interval' is -1m0s"},
		},
		{
			name: "validate should fail when wait duration in open state is negative",
			cfg: newCircuitBreakerConfig(func(breaker *config.CircuitBreaker) {
				breaker.WaitDurationInOpenState = config.Duration{Duration: -time.Second}
			}),
			expected: []string{
				"duration must not be negative: 'Config.Gateway.Routes[0].CircuitBreaker.WaitDurationInOpenState' is -1s",
			},
		},
		{
			name: "validate should fail when the circuit breaker is disabled and its settings are negative",
			cfg: newValidConfig(func(cfg *config.Config) {
				cfg.Gateway.Routes[0].CircuitBreaker = config.CircuitBreaker{
					Interval:                config.Duration{Duration: -time.Second},
					NumAllowedHalfOpenCalls: -1,
					WaitDurationInOpenState: config.Duration{Duration: -time.Second},
					MinRequestsThreshold:    -1,
				}
			}),
			expected: []string{
				"'NumAllowedHalfOpenCalls' failed on the 'min' tag",
				"'MinRequestsThreshold' failed on the 'min' tag",
				"'Config.Gateway.Routes[0].CircuitBreaker.Interval' is -1s",
				"'Config.Gateway.Routes[0].CircuitBreaker.WaitDurationInOpenState' is -1s",
			},
		},
		{
			name: "validate should name the route of a negative duration",
			cfg: newValidConfig(func(cfg *config.Config) {
				cfg.Gateway.Routes = append(cfg.Gateway.Routes, config.Route{
					ID:             "r2",
					URI:            "http://localhost:8002",
					CircuitBreaker: config.CircuitBreaker{Interval: config.Duration{Duration: -time.Second}},
				})
			}),
			expected: []string{"'Config.Gateway.Routes[1].CircuitBreaker.Interval' is -1s"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.Validate(tt.cfg)

			if len(tt.expected) == 0 {
				if err != nil {
					t.Fatalf("expected nil err actual %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error with %q actual nil", tt.expected)
			}
			if !strings.HasPrefix(err.Error(), "validate config failed: ") {
				t.Errorf("expected the error to say that the validation failed, actual %v", err)
			}
			for _, expected := range tt.expected {
				if !strings.Contains(err.Error(), expected) {
					t.Errorf("expected the error to contain %q actual %v", expected, err)
				}
			}
			// One line per broken rule: nothing else is reported.
			if lines := strings.Count(err.Error(), "\n") + 1; lines != len(tt.expected) {
				t.Errorf("expected %d errors actual %d: %v", len(tt.expected), lines, err)
			}
		})
	}
}

func TestValidate_ErrorsCanBeInspected(t *testing.T) {
	t.Run("nil config should be ErrNilConfig", func(t *testing.T) {
		if err := config.Validate(nil); !errors.Is(err, config.ErrNilConfig) {
			t.Errorf("expected %v actual %v", config.ErrNilConfig, err)
		}
	})
	t.Run("negative duration should be ErrNegativeDuration", func(t *testing.T) {
		err := config.Validate(newCircuitBreakerConfig(func(breaker *config.CircuitBreaker) {
			breaker.Interval = config.Duration{Duration: -time.Second}
		}))
		if !errors.Is(err, config.ErrNegativeDuration) {
			t.Errorf("expected %v actual %v", config.ErrNegativeDuration, err)
		}
	})
	t.Run("broken tags should be validator.ValidationErrors", func(t *testing.T) {
		err := config.Validate(newCircuitBreakerConfig(func(breaker *config.CircuitBreaker) {
			breaker.MinRequestsThreshold = -1
			breaker.Interval = config.Duration{Duration: -time.Second}
		}))
		var validationErrs validator.ValidationErrors
		if !errors.As(err, &validationErrs) {
			t.Fatalf("expected validator.ValidationErrors actual %v", err)
		}
		if len(validationErrs) != 1 || validationErrs[0].Field() != "MinRequestsThreshold" {
			t.Errorf("expected one error for MinRequestsThreshold actual %v", validationErrs)
		}
		if !errors.Is(err, config.ErrNegativeDuration) {
			t.Errorf("expected the negative duration to be reported too, actual %v", err)
		}
	})
}

// circuitBreakerSettings holds the circuit breaker settings as they are written in a
// config file, so that numbers that do not fit in every int can be tested.
type circuitBreakerSettings struct {
	interval     string
	minRequests  string
	halfOpen     string
	waitDuration string
	enabled      bool
}

func (s circuitBreakerSettings) json() string {
	return fmt.Sprintf(`{"gateway":{"routes":[{"id":"r1","uri":"http://localhost:8001","circuit-breaker":{`+
		`"enabled":%t,"interval":%q,"min-requests-threshold":%s,"failure-rate-threshold":50,`+
		`"wait-duration-in-open-state":%q,"num-allowed-half-open-calls":%s}}]}}`,
		s.enabled, s.interval, s.minRequests, s.waitDuration, s.halfOpen)
}

func (s circuitBreakerSettings) yaml() string {
	return fmt.Sprintf(`gateway:
  routes:
    - id: r1
      uri: http://localhost:8001
      circuit-breaker:
        enabled: %t
        interval: %s
        min-requests-threshold: %s
        failure-rate-threshold: 50
        wait-duration-in-open-state: %s
        num-allowed-half-open-calls: %s
`, s.enabled, s.interval, s.minRequests, s.waitDuration, s.halfOpen)
}

// The readers and Validate share one definition of a valid config: what one rejects, the
// others reject.
func TestReaders_Read_CircuitBreakerRanges(t *testing.T) {
	valid := circuitBreakerSettings{enabled: true, interval: "1m", minRequests: "20", halfOpen: "5", waitDuration: "30s"}
	with := func(change func(settings *circuitBreakerSettings)) circuitBreakerSettings {
		settings := valid
		change(&settings)
		return settings
	}
	tests := []struct {
		name string
		// expected is a part of the expected error, empty when the config is valid.
		expected string
		settings circuitBreakerSettings
	}{
		{
			name:     "read should succeed when the settings are in range",
			settings: valid,
		},
		{
			name:     "read should succeed when min requests threshold is the lowest one",
			settings: with(func(s *circuitBreakerSettings) { s.minRequests = "1" }),
		},
		{
			name:     "read should succeed when min requests threshold is the highest one",
			settings: with(func(s *circuitBreakerSettings) { s.minRequests = "4294967295" }),
		},
		{
			name:     "read should fail when min requests threshold is zero",
			settings: with(func(s *circuitBreakerSettings) { s.minRequests = "0" }),
			expected: "MinRequestsThreshold",
		},
		{
			name:     "read should fail when min requests threshold is negative",
			settings: with(func(s *circuitBreakerSettings) { s.minRequests = "-1" }),
			expected: "'MinRequestsThreshold' failed on the 'min' tag",
		},
		{
			name:     "read should fail when min requests threshold is over what the breaker can count",
			settings: with(func(s *circuitBreakerSettings) { s.minRequests = "4294967296" }),
			expected: "'MinRequestsThreshold' failed on the 'max' tag",
		},
		{
			name:     "read should succeed when num allowed half open calls is the lowest one",
			settings: with(func(s *circuitBreakerSettings) { s.halfOpen = "1" }),
		},
		{
			name:     "read should succeed when num allowed half open calls is the highest one",
			settings: with(func(s *circuitBreakerSettings) { s.halfOpen = "4294967295" }),
		},
		{
			name:     "read should fail when num allowed half open calls is zero",
			settings: with(func(s *circuitBreakerSettings) { s.halfOpen = "0" }),
			expected: "NumAllowedHalfOpenCalls",
		},
		{
			name:     "read should fail when num allowed half open calls is negative",
			settings: with(func(s *circuitBreakerSettings) { s.halfOpen = "-1" }),
			expected: "'NumAllowedHalfOpenCalls' failed on the 'min' tag",
		},
		{
			name:     "read should fail when num allowed half open calls is over what the breaker can count",
			settings: with(func(s *circuitBreakerSettings) { s.halfOpen = "4294967296" }),
			expected: "'NumAllowedHalfOpenCalls' failed on the 'max' tag",
		},
		{
			name:     "read should fail when interval is negative",
			settings: with(func(s *circuitBreakerSettings) { s.interval = "-1m" }),
			expected: "duration must not be negative: 'Config.Gateway.Routes[0].CircuitBreaker.Interval' is -1m0s",
		},
		{
			name:     "read should fail when wait duration in open state is negative",
			settings: with(func(s *circuitBreakerSettings) { s.waitDuration = "-30s" }),
			expected: "duration must not be negative: 'Config.Gateway.Routes[0].CircuitBreaker.WaitDurationInOpenState' is -30s",
		},
		{
			name: "read should succeed when the breaker is disabled and has no settings",
			settings: circuitBreakerSettings{
				enabled: false, interval: "0s", minRequests: "0", halfOpen: "0", waitDuration: "0s",
			},
		},
		{
			name: "read should fail when the breaker is disabled and min requests threshold is negative",
			settings: with(func(s *circuitBreakerSettings) {
				s.enabled = false
				s.minRequests = "-1"
			}),
			expected: "'MinRequestsThreshold' failed on the 'min' tag",
		},
		{
			name: "read should fail when the breaker is disabled and interval is negative",
			settings: with(func(s *circuitBreakerSettings) {
				s.enabled = false
				s.interval = "-1m"
			}),
			expected: "'Config.Gateway.Routes[0].CircuitBreaker.Interval' is -1m0s",
		},
	}
	for _, tt := range tests {
		readers := []struct {
			reader  config.Reader
			name    string
			input   string
			failure string
		}{
			{
				name:    "json",
				reader:  config.NewReaderJSON(validator.New()),
				input:   tt.settings.json(),
				failure: "read json config failed: ",
			},
			{
				name:    "yaml",
				reader:  config.NewReaderYAML(validator.New()),
				input:   tt.settings.yaml(),
				failure: "read yaml config failed: ",
			},
		}
		for _, reader := range readers {
			t.Run(tt.name+" ("+reader.name+")", func(t *testing.T) {
				cfg, err := reader.reader.Read([]byte(reader.input))

				if tt.expected == "" {
					if err != nil {
						t.Fatalf("expected nil err actual %v", err)
					}
					if err = config.Validate(cfg); err != nil {
						t.Errorf("expected the config that was read to be valid, actual %v", err)
					}
					return
				}
				if err == nil {
					t.Fatalf("expected an error with %q actual nil", tt.expected)
				}
				if !strings.HasPrefix(err.Error(), reader.failure) || !strings.Contains(err.Error(), tt.expected) {
					t.Errorf("expected an error with %q and %q actual %v", reader.failure, tt.expected, err)
				}
				if cfg != nil {
					t.Errorf("expected nil config actual %v", cfg)
				}
			})
		}
	}
}

// The validator given to a reader only checks the tags. Without one the reader used to
// panic: it now checks them with a validator of its own, the one Validate uses.
func TestReaders_Read_NilValidator(t *testing.T) {
	valid := circuitBreakerSettings{enabled: true, interval: "1m", minRequests: "20", halfOpen: "5", waitDuration: "30s"}
	invalid := valid
	invalid.minRequests = "-1"
	readers := []struct {
		reader config.Reader
		name   string
		valid  string
		broken string
	}{
		{name: "json", reader: config.NewReaderJSON(nil), valid: valid.json(), broken: invalid.json()},
		{name: "yaml", reader: config.NewReaderYAML(nil), valid: valid.yaml(), broken: invalid.yaml()},
	}
	for _, tt := range readers {
		t.Run(tt.name+" reader should read a valid config", func(t *testing.T) {
			cfg, err := tt.reader.Read([]byte(tt.valid))
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}
			if cfg == nil || cfg.Gateway.Routes[0].CircuitBreaker.MinRequestsThreshold != 20 {
				t.Errorf("expected the config that was written actual %v", cfg)
			}
		})
		t.Run(tt.name+" reader should reject an invalid config", func(t *testing.T) {
			_, err := tt.reader.Read([]byte(tt.broken))
			if err == nil || !strings.Contains(err.Error(), "'MinRequestsThreshold' failed on the 'min' tag") {
				t.Errorf("expected an error for min requests threshold actual %v", err)
			}
		})
	}
}
