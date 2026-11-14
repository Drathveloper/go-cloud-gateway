package config

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/httpclient"
)

func TestIsRequestSuccessful(t *testing.T) {
	tests := []struct {
		err      error
		name     string
		expected bool
	}{
		{
			name:     "nil error is a success",
			err:      nil,
			expected: true,
		},
		{
			name:     "client cancellation is not a backend failure",
			err:      context.Canceled,
			expected: true,
		},
		{
			name:     "wrapped client cancellation is not a backend failure",
			err:      fmt.Errorf("request failed: %w", context.Canceled),
			expected: true,
		},
		{
			name:     "timeout is a failure",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "backend 5xx is a failure",
			err:      httpclient.ErrInternalServer,
			expected: false,
		},
		{
			name:     "network error is a failure",
			err:      errors.New("dial tcp: connection refused"),
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRequestSuccessful(tt.err); got != tt.expected {
				t.Errorf("isRequestSuccessful(%v) = %v, expected %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestNewPoolSettings(t *testing.T) {
	defaults := poolSettings{
		dialTimeout:           DefaultTimeout,
		keepAlive:             DefaultKeepAlive,
		idleConnTimeout:       DefaultIdleConnTimeout,
		tlsHandshakeTimeout:   DefaultTimeout,
		responseHeaderTimeout: 0,
		maxIdleConns:          DefaultConns,
		maxIdleConnsPerHost:   DefaultConns,
		maxConnsPerHost:       DefaultConns,
	}
	tests := []struct {
		pool     *Pool
		name     string
		expected poolSettings
	}{
		{
			name:     "new pool settings should return defaults when pool is nil",
			pool:     nil,
			expected: defaults,
		},
		{
			name: "new pool settings should return pool values when pool is set",
			pool: &Pool{
				Timeout:               &Duration{Duration: time.Second},
				KeepAlive:             &Duration{Duration: 2 * time.Second},
				IdleConnTimeout:       &Duration{Duration: 3 * time.Second},
				TLSHandshakeTimeout:   &Duration{Duration: 4 * time.Second},
				ResponseHeaderTimeout: &Duration{Duration: 5 * time.Second},
				MaxIdleConns:          6,
				MaxIdleConnsPerHost:   7,
				MaxConnsPerHost:       8,
			},
			expected: poolSettings{
				dialTimeout:           time.Second,
				keepAlive:             2 * time.Second,
				idleConnTimeout:       3 * time.Second,
				tlsHandshakeTimeout:   4 * time.Second,
				responseHeaderTimeout: 5 * time.Second,
				maxIdleConns:          6,
				maxIdleConnsPerHost:   7,
				maxConnsPerHost:       8,
			},
		},
		{
			name: "new pool settings should keep default durations when pool leaves them out",
			pool: &Pool{
				MaxIdleConns:        6,
				MaxIdleConnsPerHost: 7,
				MaxConnsPerHost:     8,
			},
			expected: poolSettings{
				dialTimeout:           DefaultTimeout,
				keepAlive:             DefaultKeepAlive,
				idleConnTimeout:       DefaultIdleConnTimeout,
				tlsHandshakeTimeout:   DefaultTimeout,
				responseHeaderTimeout: 0,
				maxIdleConns:          6,
				maxIdleConnsPerHost:   7,
				maxConnsPerHost:       8,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := newPoolSettings(tt.pool)

			if tt.expected != actual {
				t.Errorf("expected %v actual %v", tt.expected, actual)
			}
		})
	}
}
