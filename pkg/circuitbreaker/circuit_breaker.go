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
type CircuitBreaker[T any] struct {
	expiry        time.Time
	readyToTrip   func(counts Counts) bool
	isSuccessful  func(err error) bool
	onStateChange func(name string, from State, to State)
	name          string
	interval      time.Duration
	timeout       time.Duration
	state         State
	generation    uint64
	counts        Counts
	mutex         sync.Mutex
	maxRequests   uint32
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
	defer cb.mutex.Unlock()

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
func (cb *CircuitBreaker[T]) Execute(req func() (T, error)) (T, error) { //nolint:ireturn
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
	defer cb.mutex.Unlock()

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
	defer cb.mutex.Unlock()

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
		cb.onStateChange(cb.name, prev, state)
	}
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
