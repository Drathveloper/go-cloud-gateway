package config

import "time"

type Config struct {
	Gateway Gateway `validate:"required" json:"gateway" yaml:"gateway"`
}

type Gateway struct {
	HTTPClient    *HTTPClient         `json:"httpclient"     yaml:"httpclient"`
	Routes        []Route             `validate:"required,min=1,dive" json:"routes" yaml:"routes"`
	GlobalFilters []ParameterizedItem `validate:"dive"                json:"global-filters" yaml:"global-filters"`
	GlobalTimeout Duration            `json:"global-timeout" yaml:"global-timeout"`
}

type Route struct {
	ID         string              `validate:"required" json:"id"  yaml:"id"`
	URI        string              `validate:"required" json:"uri" yaml:"uri"`
	Predicates []ParameterizedItem `validate:"dive"     json:"predicates" yaml:"predicates"`
	Filters    []ParameterizedItem `validate:"dive"     json:"filters"    yaml:"filters"`
	Timeout    Duration            `json:"timeout" yaml:"timeout"`
}

type ParameterizedItem struct {
	Args map[string]any `json:"args" yaml:"args"`
	Name string         `validate:"required" json:"name" yaml:"name"`
}

type HTTPClient struct {
	MTLS              *MTLS `json:"mtls"                yaml:"mtls"`
	Pool              *Pool `json:"pool"                yaml:"pool"`
	InsecureTLSVerify bool  `json:"insecure-tls-verify" yaml:"insecure-tls-verify"`
	EnableHTTP2       bool  `json:"enable-http2"        yaml:"enable-http2"`
}

type Pool struct {
	Timeout             *Duration `validate:"required" json:"timeout"                 yaml:"timeout"`
	KeepAlive           *Duration `validate:"required" json:"keep-alive"              yaml:"keep-alive"`
	IdleConnTimeout     *Duration `validate:"required" json:"idle-conn-timeout"       yaml:"idle-conn-timeout"`
	TLSHandshakeTimeout *Duration `validate:"required" json:"tls-handshake-timeout"   yaml:"tls-handshake-timeout"`
	MaxIdleConns        int       `validate:"required" json:"max-idle-conns"          yaml:"max-idle-conns"`
	MaxIdleConnsPerHost int       `validate:"required" json:"max-idle-conns-per-host" yaml:"max-idle-conns-per-host"`
	MaxConnsPerHost     int       `validate:"required" json:"max-conns-per-host"      yaml:"max-conns-per-host"`
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
