package common //nolint:revive

import "time"

// TimeProvider is a time provider. Implemented to ease the testing.
type TimeProvider interface {
	// Now returns the current time.
	Now() time.Time
}

// RealTime is a real time provider.
type RealTime struct{}

// Now returns the real current time.
func (t *RealTime) Now() time.Time {
	return time.Now()
}
