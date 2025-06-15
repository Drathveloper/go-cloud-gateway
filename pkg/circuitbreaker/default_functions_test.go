package circuitbreaker_test

import (
	"io"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/circuitbreaker"
)

func TestDefaultReadyToTrip(t *testing.T) {
	tests := []struct {
		name        string
		minRequests int
		failureRate int
		counts      circuitbreaker.Counts
		expected    bool
	}{
		{
			name:        "default ready to trip should return false when min requests still not overpassed",
			minRequests: 1,
			failureRate: 100,
			counts:      circuitbreaker.Counts{},
			expected:    false,
		},
		{
			name:        "default ready to trip should return true when error rate equals threshold",
			minRequests: 1,
			failureRate: 100,
			counts: circuitbreaker.Counts{
				Requests:             1,
				TotalSuccesses:       0,
				TotalFailures:        1,
				ConsecutiveSuccesses: 0,
				ConsecutiveFailures:  0,
			},
			expected: true,
		},
		{
			name:        "default ready to trip should return true when error rate above threshold",
			minRequests: 1,
			failureRate: 40,
			counts: circuitbreaker.Counts{
				Requests:       2,
				TotalSuccesses: 0,
				TotalFailures:  1,
			},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readyToTripFunc := circuitbreaker.DefaultReadyToTrip(tt.minRequests, tt.failureRate)

			result := readyToTripFunc(tt.counts)

			if tt.expected != result {
				t.Errorf("expected %t actual %t", tt.expected, result)
			}
		})
	}
}

func TestDefaultIsSuccessful(t *testing.T) {
	tests := []struct {
		err         error
		expectedErr error
		name        string
		expected    bool
	}{
		{
			name:     "default is successful should return true when error is nil",
			err:      nil,
			expected: true,
		},
		{
			name:        "default is successful should return true when error is not expected error",
			expectedErr: io.ErrClosedPipe,
			err:         io.EOF,
			expected:    true,
		},
		{
			name:        "default is successful should return false when error is expected error",
			expectedErr: io.EOF,
			err:         io.EOF,
			expected:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			successFunc := circuitbreaker.DefaultIsSuccessful(tt.expectedErr)

			result := successFunc(tt.err)

			if tt.expected != result {
				t.Errorf("expected %t actual %t", tt.expected, result)
			}
		})
	}
}
