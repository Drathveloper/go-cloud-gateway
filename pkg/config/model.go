package config

import "time"

type Config struct {
	Gateway       Gateway  `validate:"required" json:"gateway" yaml:"gateway"`
	GlobalTimeout Duration `json:"global-timeout" yaml:"global-timeout"`
}

type Gateway struct {
	Routes        []Route             `validate:"required,dive" json:"routes" yaml:"routes"`
	GlobalFilters []ParameterizedItem `validate:"dive"          json:"global-filters" yaml:"global-filters"`
	HTTPClient    *HTTPClientConfig   `json:"httpclient" yaml:"httpclient"`
}

type Route struct {
	ID         string              `validate:"required" json:"id"  yaml:"id"`
	URI        string              `validate:"required" json:"uri" yaml:"uri"`
	Predicates []ParameterizedItem `validate:"dive"     json:"predicates" yaml:"predicates"`
	Filters    []ParameterizedItem `validate:"dive"     json:"filters"    yaml:"filters"`
	Timeout    time.Duration       `json:"timeout"    yaml:"timeout"`
}

type ParameterizedItem struct {
	Name string         `validate:"required" json:"name" yaml:"name"`
	Args map[string]any `json:"args" yaml:"args"`
}

type HTTPClientConfig struct {
	ConnectTimeout      Duration `json:"connect-timeout"         yaml:"connect-timeout"`
	MaxIdleConns        int      `json:"max-idle-conns"          yaml:"max-idle-conns"`
	MaxIdleConnsPerHost int      `json:"max-idle-conns-per-host" yaml:"max-idle-conns-per-host"`
	MaxConnsPerHost     int      `json:"max-conns-per-host"      yaml:"max-conns-per-host"`
	IdleConnTimeout     Duration `json:"idle-conn-timeout"       yaml:"idle-conn-timeout"`
	TLSHandshakeTimeout Duration `json:"tls-handshake-timeout"   yaml:"tls-handshake-timeout"`
}

type Duration struct {
	time.Duration
}
