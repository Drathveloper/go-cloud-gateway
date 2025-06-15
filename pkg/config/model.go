package config

import "time"

// Config represents the wrapper for gateway config.
type Config struct {
	Gateway Gateway `json:"gateway" yaml:"gateway" validate:"required"`
}

// Gateway represents the gateway config.
type Gateway struct {
	HTTPClient    *HTTPClient         `json:"httpclient"     yaml:"httpclient"`
	Routes        []Route             `json:"routes"         yaml:"routes"         validate:"required,min=1,dive"`
	GlobalFilters []ParameterizedItem `json:"global-filters" yaml:"global-filters" validate:"dive"`
	GlobalTimeout Duration            `json:"global-timeout" yaml:"global-timeout"`
}

// Route represents the gateway route config.
type Route struct {
	ID             string              `json:"id"              yaml:"id"              validate:"required"`
	URI            string              `json:"uri"             yaml:"uri"             validate:"required"`
	Predicates     []ParameterizedItem `json:"predicates"      yaml:"predicates"      validate:"dive"`
	Filters        []ParameterizedItem `json:"filters"         yaml:"filters"         validate:"dive"`
	Timeout        Duration            `json:"timeout"         yaml:"timeout"`
	CircuitBreaker CircuitBreaker      `json:"circuit-breaker" yaml:"circuit-breaker"`
}

// CircuitBreaker represents the gateway circuit breaker config.
//
// The circuit breaker configuration fields are required if the circuit breaker is enabled.
type CircuitBreaker struct {
	Enabled                 bool     `json:"enabled"                     yaml:"enabled"`
	Interval                Duration `json:"interval"                    yaml:"interval"                    validate:"required_if=Enabled true"` //nolint:lll
	FailureRateThreshold    int      `json:"failure-rate-threshold"      yaml:"failure-rate-threshold"      validate:"required_if=Enabled true"` //nolint:lll
	NumAllowedHalfOpenCalls int      `json:"num-allowed-half-open-calls" yaml:"num-allowed-half-open-calls" validate:"required_if=Enabled true"` //nolint:lll
	WaitDurationInOpenState Duration `json:"wait-duration-in-open-state" yaml:"wait-duration-in-open-state" validate:"required_if=Enabled true"` //nolint:lll
	MinRequestsThreshold    int      `json:"min-requests-threshold"      yaml:"min-requests-threshold"      validate:"required_if=Enabled true"` //nolint:lll
}

// ParameterizedItem represents the gateway predicate or filter config.
//
// The args field is a map of string to any. The key is the name of the argument.
// The value is the value of the argument.
//
// The name field is the name of the predicate or filter.
//
// The name field is required.
type ParameterizedItem struct {
	Args map[string]any `json:"args" yaml:"args"`
	Name string         `json:"name" yaml:"name" validate:"required"`
}

// HTTPClient represents the gateway http client config.
type HTTPClient struct {
	MTLS              *MTLS `json:"mtls"                yaml:"mtls"`
	Pool              *Pool `json:"pool"                yaml:"pool"`
	InsecureTLSVerify bool  `json:"insecure-tls-verify" yaml:"insecure-tls-verify"`
	EnableHTTP2       bool  `json:"enable-http2"        yaml:"enable-http2"`
}

// Pool represents the gateway http client pool config.
//
// The fields are required if the pool is customized.
type Pool struct {
	Timeout             *Duration `json:"timeout"                 yaml:"timeout"                 validate:"required"`
	KeepAlive           *Duration `json:"keep-alive"              yaml:"keep-alive"              validate:"required"`
	IdleConnTimeout     *Duration `json:"idle-conn-timeout"       yaml:"idle-conn-timeout"       validate:"required"`
	TLSHandshakeTimeout *Duration `json:"tls-handshake-timeout"   yaml:"tls-handshake-timeout"   validate:"required"`
	MaxIdleConns        int       `json:"max-idle-conns"          yaml:"max-idle-conns"          validate:"required"`
	MaxIdleConnsPerHost int       `json:"max-idle-conns-per-host" yaml:"max-idle-conns-per-host" validate:"required"`
	MaxConnsPerHost     int       `json:"max-conns-per-host"      yaml:"max-conns-per-host"      validate:"required"`
}

// MTLS represents the gateway http client mtls config.
//
// The fields are required if mtls is enabled.
type MTLS struct {
	Enabled *bool  `json:"enabled" yaml:"enabled" validate:"required"`
	CA      string `json:"ca"      yaml:"ca"      validate:"required_if=Enabled true"`
	Cert    string `json:"cert"    yaml:"cert"    validate:"required_if=Enabled true"`
	Key     string `json:"key"     yaml:"key"     validate:"required_if=Enabled true"`
}

// Duration represents a time.Duration with a custom unmarshaler.
//
// The unmarshaler supports unmarshaling of float64 and string values.
//
// The unmarshaler supports unmarshaling of the following formats:
//
// 1. 30s
// 2. 30.
type Duration struct {
	time.Duration
}
