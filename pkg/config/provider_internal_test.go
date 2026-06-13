package config

import (
	"context"
	"errors"
	"fmt"
	"testing"

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
