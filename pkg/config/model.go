package config

import "time"

type Config struct {
	Gateway Gateway `validate:"required" json:"gateway" yaml:"gateway"`
}

type Gateway struct {
	Routes        []Route             `validate:"required,min=1,dive" json:"routes" yaml:"routes"`
	GlobalFilters []ParameterizedItem `validate:"dive"                json:"global-filters" yaml:"global-filters"`
	GlobalTimeout Duration            `json:"global-timeout" yaml:"global-timeout"`
	HTTPClient    *HTTPClient         `json:"httpclient"     yaml:"httpclient"`
}

type Route struct {
	ID         string              `validate:"required" json:"id"  yaml:"id"`
	URI        string              `validate:"required" json:"uri" yaml:"uri"`
	Predicates []ParameterizedItem `validate:"dive"     json:"predicates" yaml:"predicates"`
	Filters    []ParameterizedItem `validate:"dive"     json:"filters"    yaml:"filters"`
	Timeout    Duration            `json:"timeout" yaml:"timeout"`
}

type ParameterizedItem struct {
	Name string         `validate:"required" json:"name" yaml:"name"`
	Args map[string]any `json:"args" yaml:"args"`
}

type HTTPClient struct {
	MTLS              *MTLS `json:"mtls"                yaml:"mtls"`
	Pool              *Pool `json:"pool"                yaml:"pool"`
	InsecureTLSVerify bool  `json:"insecure-tls-verify" yaml:"insecure-tls-verify"`
}

type Pool struct {
	ConnectTimeout      *Duration `validate:"required" json:"connect-timeout"         yaml:"connect-timeout"`
	MaxIdleConns        int       `validate:"required" json:"max-idle-conns"          yaml:"max-idle-conns"`
	MaxIdleConnsPerHost int       `validate:"required" json:"max-idle-conns-per-host" yaml:"max-idle-conns-per-host"`
	MaxConnsPerHost     int       `validate:"required" json:"max-conns-per-host"      yaml:"max-conns-per-host"`
	IdleConnTimeout     *Duration `validate:"required" json:"idle-conn-timeout"       yaml:"idle-conn-timeout"`
	TLSHandshakeTimeout *Duration `validate:"required" json:"tls-handshake-timeout"   yaml:"tls-handshake-timeout"`
}

type MTLS struct {
	Enabled *bool  `validate:"required"                json:"enabled" yaml:"enabled"`
	CA      string `validate:"required_if=Enabled true" json:"ca" yaml:"ca"`
	Cert    string `validate:"required_if=Enabled true" json:"cert" yaml:"cert"`
	Key     string `validate:"required_if=Enabled true" json:"key" yaml:"key"`
}

type Duration struct {
	time.Duration
}
