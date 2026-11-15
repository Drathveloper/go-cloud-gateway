package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

const (
	defaultInterval            = time.Duration(0) * time.Second
	defaultTimeout             = time.Duration(60) * time.Second
	defaultConsecutiveFailures = 5
)

// ErrHalfOpenRequestExceeded is returned when the circuit breaker is half-open and the number of requests is over the
// max number of request allowed in half-open state.
var ErrHalfOpenRequestExceeded = errors.New("too many requests while circuit breaker is half-open")

// ErrOpenState is returned when the circuit breaker is open.
var ErrOpenState = errors.New("circuit breaker is open")

// CircuitBreaker is a state machine to prevent sending requests that are likely to fail.
//
// The mutex guards the state, the counters and the delivery of the state changes
// (pending and notifying). It is never held while user code that may block runs: see
// Settings.OnStateChange for how the callback is called.
type CircuitBreaker[T any] struct {
	expiry        time.Time
	readyToTrip   func(counts Counts) bool
	isSuccessful  func(err error) bool
	onStateChange func(name string, from State, to State)
	name          string
	// pending holds the state changes OnStateChange has not been called for yet.
	pending     pendingChanges
	interval    time.Duration
	timeout     time.Duration
	state       State
	generation  uint64
	counts      Counts
	mutex       sync.Mutex
	maxRequests uint32
	// notifying is true while a goroutine is calling OnStateChange for the pending
	// state changes. It makes that goroutine the only one that calls it.
	notifying bool
}

// NewCircuitBreaker returns a new CircuitBreaker configured with the given Settings.
func NewCircuitBreaker[T any](settings Settings) *CircuitBreaker[T] {
	circuitBreaker := new(CircuitBreaker[T])

	circuitBreaker.name = settings.Name
	circuitBreaker.onStateChange = settings.OnStateChange

	if settings.MaxRequests == 0 {
		circuitBreaker.maxRequests = 1
	} else {
		circuitBreaker.maxRequests = settings.MaxRequests
	}

	if settings.Interval <= 0 {
		circuitBreaker.interval = defaultInterval
	} else {
		circuitBreaker.interval = settings.Interval
	}

	if settings.Timeout <= 0 {
		circuitBreaker.timeout = defaultTimeout
	} else {
		circuitBreaker.timeout = settings.Timeout
	}

	if settings.ReadyToTrip == nil {
		circuitBreaker.readyToTrip = defaultReadyToTrip
	} else {
		circuitBreaker.readyToTrip = settings.ReadyToTrip
	}

	if settings.IsSuccessful == nil {
		circuitBreaker.isSuccessful = defaultIsSuccessful
	} else {
		circuitBreaker.isSuccessful = settings.IsSuccessful
	}
	circuitBreaker.toNewGeneration(time.Now())

	return circuitBreaker
}

func defaultReadyToTrip(counts Counts) bool {
	return counts.ConsecutiveFailures > defaultConsecutiveFailures
}

func defaultIsSuccessful(err error) bool {
	return err == nil
}

// Name returns the name of the CircuitBreaker.
func (cb *CircuitBreaker[T]) Name() string {
	return cb.name
}

// State returns the current state of the CircuitBreaker.
func (cb *CircuitBreaker[T]) State() State {
	cb.mutex.Lock()
	defer cb.unlockAndNotify()

	now := time.Now()
	state, _ := cb.currentState(now)
	return state
}

// Counts returns internal counters.
func (cb *CircuitBreaker[T]) Counts() Counts {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	return cb.counts
}

// Execute runs the given request if the CircuitBreaker accepts it.
// Execute returns an error instantly if the CircuitBreaker rejects the request.
// Otherwise, Execute returns the result of the request.
// If a panic occurs in the request, the CircuitBreaker handles it as an error
// and causes the same panic again.
//
//nolint:ireturn // generic return is required for CircuitBreaker design
func (cb *CircuitBreaker[T]) Execute(req func() (T, error)) (T, error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		var defaultValue T
		return defaultValue, err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result, err := req()
	cb.afterRequest(generation, cb.isSuccessful(err))
	return result, err
}

func (cb *CircuitBreaker[T]) beforeRequest() (uint64, error) {
	cb.mutex.Lock()
	defer cb.unlockAndNotify()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		return generation, ErrOpenState
	} else if state == StateHalfOpen && cb.counts.Requests >= cb.maxRequests {
		return generation, ErrHalfOpenRequestExceeded
	}

	cb.counts.onRequest()
	return generation, nil
}

func (cb *CircuitBreaker[T]) afterRequest(before uint64, success bool) {
	cb.mutex.Lock()
	defer cb.unlockAndNotify()

	now := time.Now()
	state, generation := cb.currentState(now)
	if generation != before {
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

func (cb *CircuitBreaker[T]) onSuccess(state State, now time.Time) {
	switch state {
	case StateClosed:
		cb.counts.onSuccess()
	case StateHalfOpen:
		cb.counts.onSuccess()
		if cb.counts.ConsecutiveSuccesses >= cb.maxRequests {
			cb.setState(StateClosed, now)
		}
	case StateOpen: // StateOpen: do nothing
	default:
	}
}

func (cb *CircuitBreaker[T]) onFailure(state State, now time.Time) {
	switch state {
	case StateClosed:
		cb.counts.onFailure()
		if cb.readyToTrip(cb.counts) {
			cb.setState(StateOpen, now)
		}
	case StateHalfOpen:
		cb.setState(StateOpen, now)
	case StateOpen: // StateOpen: do nothing
	default:
	}
}

func (cb *CircuitBreaker[T]) currentState(now time.Time) (State, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	case StateHalfOpen: // StateHalfOpen: do nothing
	default:
	}
	return cb.state, cb.generation
}

func (cb *CircuitBreaker[T]) setState(state State, now time.Time) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state

	cb.toNewGeneration(now)

	if cb.onStateChange != nil {
		// The callback is not called here, where the mutex is held: the change is queued
		// and unlockAndNotify delivers it once the mutex is released.
		cb.pending.push(pendingChange{from: prev, to: state})
	}
}

// unlockAndNotify releases the mutex, which the caller must hold, and then calls
// OnStateChange for the state changes queued while it was held.
//
// The callback runs without the mutex so that a slow one only delays the goroutine that
// delivers it, not every request of the breaker, and so that it can call back into the
// breaker. Delivery is still serialized: the goroutine that finds pending changes and
// nobody delivering them takes the role, and whoever changes the state meanwhile leaves
// its change in the queue for that goroutine. The changes therefore reach the callback
// one at a time and in the order they happened, without a goroutine of their own.
func (cb *CircuitBreaker[T]) unlockAndNotify() {
	deliver := !cb.notifying && !cb.pending.empty()
	if deliver {
		cb.notifying = true
	}
	cb.mutex.Unlock()
	if deliver {
		cb.notifyStateChanges()
	}
}

// notifyStateChanges calls OnStateChange for the pending state changes until none is
// left, including the ones queued while it runs. The caller must have set notifying.
func (cb *CircuitBreaker[T]) notifyStateChanges() {
	drained := false
	defer func() {
		if drained {
			return
		}
		// The callback panicked. The role is given up, or no state change would be
		// delivered again: the next call into the breaker resumes with what is queued.
		cb.mutex.Lock()
		cb.notifying = false
		cb.mutex.Unlock()
	}()
	for {
		change, ok := cb.nextStateChange()
		if !ok {
			drained = true
			return
		}
		cb.onStateChange(cb.name, change.from, change.to)
	}
}

// nextStateChange returns the oldest pending state change. When there is none it ends
// the delivery under the same lock, so that a change queued right after is delivered by
// the goroutine that queued it instead of being left behind.
func (cb *CircuitBreaker[T]) nextStateChange() (pendingChange, bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	change, ok := cb.pending.pop()
	if !ok {
		cb.notifying = false
	}
	return change, ok
}

func (cb *CircuitBreaker[T]) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts.clear()

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.interval)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.timeout)
	case StateHalfOpen: // StateHalfOpen
		cb.expiry = zero
	default:
		cb.expiry = zero
	}
}
