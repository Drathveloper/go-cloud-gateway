package circuitbreaker

import (
	"time"
)

// Settings configures CircuitBreaker:
//
// Name is the name of the CircuitBreaker.
//
// MaxRequests is the maximum number of requests allowed to pass through
// when the CircuitBreaker is half-open.
// If MaxRequests is 0, the CircuitBreaker allows only 1 request.
//
// Interval is the cyclic period of the closed state
// for the CircuitBreaker to clear the internal Counts.
// If Interval is less than or equal to 0, the CircuitBreaker doesn't clear internal Counts during the closed state.
//
// Timeout is the period of the open state,
// after which the state of the CircuitBreaker becomes half-open.
// If Timeout is less than or equal to 0, the timeout value of the CircuitBreaker is set to 60 seconds.
//
// ReadyToTrip is called with a copy of Counts whenever a request fails in the closed state.
// If ReadyToTrip returns true, the CircuitBreaker will be placed into the open state.
// If ReadyToTrip is nil, the default ReadyToTrip is used.
// Default ReadyToTrip returns true when the number of consecutive failures is more than 5.
//
// OnStateChange is called whenever the state of the CircuitBreaker changes.
// It is called without the lock of the CircuitBreaker held, one call at a time and in the
// order of the state changes. The goroutine whose call changed the state makes the call
// before it returns, unless another goroutine is inside the callback: the state change
// is then queued, and that other goroutine delivers it once the callback returns.
// So while a callback has not returned, the CircuitBreaker keeps serving requests, the
// state changes are reported late, State may already be past the reported one, and the
// goroutine inside the callback stays until every queued state change is delivered.
// The queue holds 16 state changes: when a callback takes so long that it fills up, the
// oldest queued ones are dropped and never reported.
// The callback may call the CircuitBreaker. A state change it causes is delivered after
// it returns.
//
// IsSuccessful is called with the error returned from a request.
// If IsSuccessful returns true, the error is counted as a success.
// Otherwise, the error is counted as a failure.
// If IsSuccessful is nil, default IsSuccessful is used, which returns false for all non-nil errors.
type Settings struct {
	ReadyToTrip   func(counts Counts) bool
	OnStateChange func(name string, from State, to State)
	IsSuccessful  func(err error) bool
	Name          string
	Interval      time.Duration
	Timeout       time.Duration
	MaxRequests   uint32
}
