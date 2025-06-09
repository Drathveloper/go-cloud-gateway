package config

import "time"

const (
	DefaultTimeout         = 10 * time.Second
	DefaultKeepAlive       = 30 * time.Second
	ContinueDefaultTimeout = 0 * time.Second
	DefaultIdleConnTimeout = 60 * time.Second
	DefaultConns           = 200
)
