package circuitbreaker

import (
	"errors"
)

const divideByPercentage = 100.0

// DefaultReadyToTrip returns true if the number of consecutive failures is higher than the failure rate threshold.
// If there are not enough requests, the function returns always false.
func DefaultReadyToTrip(minRequestsThreshold, failureRateThreshold int) func(counts Counts) bool {
	return func(counts Counts) bool {
		if int(counts.Requests) < minRequestsThreshold {
			return false
		}
		errorRate := float64(counts.TotalFailures) / float64(counts.Requests)
		threshold := float64(failureRateThreshold) / divideByPercentage
		return errorRate >= threshold
	}
}

// DefaultIsSuccessful returns true if the error is not nil and is not the expected error.
//
// If the error is nil, the function returns true.
// If the error is not nil and is the expected error, the function returns false.
// If the error is not nil and is not the expected error, the function returns true.
func DefaultIsSuccessful(expectedErr error) func(err error) bool {
	return func(err error) bool {
		if err == nil {
			return true
		}
		switch {
		case errors.Is(err, expectedErr):
			return false
		default:
			return true
		}
	}
}
