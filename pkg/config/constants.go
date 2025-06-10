package config

import "time"

const (
	// DefaultTimeout is the default timeout for the gateway.
	DefaultTimeout = 10 * time.Second

	// DefaultKeepAlive is the default keep alive for the gateway.
	DefaultKeepAlive = 30 * time.Second

	// ContinueDefaultTimeout is the default continue timeout for the gateway.
	ContinueDefaultTimeout = 0 * time.Second

	// DefaultIdleConnTimeout is the default idle connection timeout for the gateway.
	DefaultIdleConnTimeout = 60 * time.Second

	// DefaultConns is the default number of connections for the gateway.
	DefaultConns = 200
)
