package circuitbreaker_test

import (
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/drathveloper/go-cloud-gateway/pkg/circuitbreaker"
)

// watchdog is how long a test waits for a call that must not block. Only a failing test
// waits that long: it fails instead of hanging until the test binary times out.
const watchdog = 5 * time.Second

// openPeriod is the Settings.Timeout of the breakers that run on the fake clock.
const openPeriod = 30 * time.Second

var errBackend = errors.New("backend failed")

// stateChange is a call to the OnStateChange callback.
type stateChange struct {
	name string
	from circuitbreaker.State
	to   circuitbreaker.State
}

// stateChanges records the calls to the OnStateChange callback.
type stateChanges struct {
	calls []stateChange
	mutex sync.Mutex
}

// add records a call and returns how many have been recorded.
func (c *stateChanges) add(name string, from, to circuitbreaker.State) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.calls = append(c.calls, stateChange{name: name, from: from, to: to})
	return len(c.calls)
}

func (c *stateChanges) all() []stateChange {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return append([]stateChange(nil), c.calls...)
}

func tripOnFirstFailure(circuitbreaker.Counts) bool {
	return true
}

// succeeding is a request that succeeds.
func succeeding() (bool, error) {
	return true, nil
}

// failing is a request that fails with errBackend.
func failing() (bool, error) {
	return false, errBackend
}

// execute runs the request through the breaker on another goroutine. The error Execute
// returns is sent to the returned channel.
func execute(breaker *circuitbreaker.CircuitBreaker[bool], request func() (bool, error)) <-chan error {
	done := make(chan error, 1)
	go func() {
		_, err := breaker.Execute(request)
		done <- err
	}()
	return done
}

// receive returns the error sent to ch, or fails the test when nothing arrives.
func receive(t *testing.T, ch <-chan error, what string) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(watchdog):
		require.FailNowf(t, "blocked", "%s did not return within %s", what, watchdog)
		return nil
	}
}

// The callback used to run with the mutex of the breaker held, so a callback that was
// slow, such as a log handler writing to a stalled output, held up every request of the
// breaker.
func TestCircuitBreaker_OnStateChange_BlockedCallbackDoesNotBlockTheBreaker(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	breaker := circuitbreaker.NewCircuitBreaker[bool](circuitbreaker.Settings{
		Name:        "cb",
		Timeout:     time.Hour,
		ReadyToTrip: tripOnFirstFailure,
		OnStateChange: func(string, circuitbreaker.State, circuitbreaker.State) {
			entered <- struct{}{}
			<-release
		},
	})
	tripped := execute(breaker, failing)
	select {
	case <-entered:
	case <-time.After(watchdog):
		require.FailNow(t, "the state change callback was not called")
	}

	type observation struct {
		err    error
		counts circuitbreaker.Counts
		state  circuitbreaker.State
	}
	observed := make(chan observation, 1)
	go func() {
		state := breaker.State()
		counts := breaker.Counts()
		_, err := breaker.Execute(succeeding)
		observed <- observation{state: state, counts: counts, err: err}
	}()

	var actual observation
	blocked := false
	select {
	case actual = <-observed:
	case <-time.After(watchdog):
		blocked = true
	}
	close(release)

	require.ErrorIs(t, receive(t, tripped, "the request that opened the breaker"), errBackend)
	require.False(t, blocked, "the breaker was blocked while the state change callback ran")
	assert.Equal(t, circuitbreaker.StateOpen, actual.state)
	assert.Equal(t, circuitbreaker.Counts{}, actual.counts)
	require.ErrorIs(t, actual.err, circuitbreaker.ErrOpenState)
}

func TestCircuitBreaker_OnStateChange_DeliversTheChangesQueuedDuringACallbackInOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var changes stateChanges
		release := make(chan struct{})
		breaker := circuitbreaker.NewCircuitBreaker[bool](circuitbreaker.Settings{
			Name:        "cb",
			Timeout:     openPeriod,
			ReadyToTrip: tripOnFirstFailure,
			OnStateChange: func(name string, from, to circuitbreaker.State) {
				if changes.add(name, from, to) == 1 {
					<-release
				}
			},
		})
		opened := stateChange{name: "cb", from: circuitbreaker.StateClosed, to: circuitbreaker.StateOpen}
		tripped := execute(breaker, failing)
		synctest.Wait()
		require.Equal(t, []stateChange{opened}, changes.all())

		// The breaker goes through a whole cycle while the first callback has not returned.
		time.Sleep(openPeriod + time.Second)
		require.Equal(t, circuitbreaker.StateHalfOpen, breaker.State())
		_, err := breaker.Execute(succeeding)
		require.NoError(t, err)
		require.Equal(t, circuitbreaker.StateClosed, breaker.State())
		_, err = breaker.Execute(failing)
		require.ErrorIs(t, err, errBackend)
		require.Equal(t, circuitbreaker.StateOpen, breaker.State())
		require.Equal(t, []stateChange{opened}, changes.all(), "a callback ran while the previous one had not returned")

		close(release)

		// The goroutine that was inside the callback delivers the rest before it returns.
		require.ErrorIs(t, <-tripped, errBackend)
		assert.Equal(t, []stateChange{
			opened,
			{name: "cb", from: circuitbreaker.StateOpen, to: circuitbreaker.StateHalfOpen},
			{name: "cb", from: circuitbreaker.StateHalfOpen, to: circuitbreaker.StateClosed},
			opened,
		}, changes.all())
	})
}

// The mutex of the breaker is not reentrant: a callback that read the state of the
// breaker deadlocked while the callback ran with it held.
func TestCircuitBreaker_OnStateChange_CallbackMayCallTheBreaker(t *testing.T) {
	const shortOpenPeriod = time.Millisecond
	type observation struct {
		err   error
		state circuitbreaker.State
	}
	var (
		breaker  *circuitbreaker.CircuitBreaker[bool]
		changes  stateChanges
		observed []observation
		running  int
		nested   bool
	)
	breaker = circuitbreaker.NewCircuitBreaker[bool](circuitbreaker.Settings{
		Name:        "cb",
		Timeout:     shortOpenPeriod,
		ReadyToTrip: tripOnFirstFailure,
		OnStateChange: func(name string, from, to circuitbreaker.State) {
			running++
			defer func() { running-- }()
			nested = nested || running > 1
			changes.add(name, from, to)
			if to != circuitbreaker.StateOpen {
				return
			}
			// The open period is over when the state is read, which moves the breaker
			// to half-open from inside the callback. Sleeping for longer than needed
			// cannot change the outcome.
			time.Sleep(5 * shortOpenPeriod)
			state := breaker.State()
			_, err := breaker.Execute(succeeding)
			observed = append(observed, observation{state: state, err: err})
		},
	})

	err := receive(t, execute(breaker, failing), "the request that opened the breaker")

	require.ErrorIs(t, err, errBackend)
	// The request made by the callback is the half-open probe, and it closes the breaker.
	assert.Equal(t, []observation{{state: circuitbreaker.StateHalfOpen}}, observed)
	assert.Equal(t, []stateChange{
		{name: "cb", from: circuitbreaker.StateClosed, to: circuitbreaker.StateOpen},
		{name: "cb", from: circuitbreaker.StateOpen, to: circuitbreaker.StateHalfOpen},
		{name: "cb", from: circuitbreaker.StateHalfOpen, to: circuitbreaker.StateClosed},
	}, changes.all())
	assert.False(t, nested, "a state change caused by the callback was delivered before the callback returned")
	assert.Equal(t, circuitbreaker.StateClosed, breaker.State())
}

// A callback that panics must not leave the breaker believing that somebody is still
// delivering the state changes: no later change would reach the callback.
func TestCircuitBreaker_OnStateChange_PanicDoesNotStopLaterDeliveries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var changes stateChanges
		breaker := circuitbreaker.NewCircuitBreaker[bool](circuitbreaker.Settings{
			Name:        "cb",
			Timeout:     openPeriod,
			ReadyToTrip: tripOnFirstFailure,
			OnStateChange: func(name string, from, to circuitbreaker.State) {
				if changes.add(name, from, to) == 1 {
					panic("callback failed")
				}
			},
		})

		assert.PanicsWithValue(t, "callback failed", func() { _, _ = breaker.Execute(failing) })

		_, err := breaker.Execute(succeeding)
		require.ErrorIs(t, err, circuitbreaker.ErrOpenState)
		time.Sleep(openPeriod + time.Second)
		_, err = breaker.Execute(succeeding)
		require.NoError(t, err)
		assert.Equal(t, []stateChange{
			{name: "cb", from: circuitbreaker.StateClosed, to: circuitbreaker.StateOpen},
			{name: "cb", from: circuitbreaker.StateOpen, to: circuitbreaker.StateHalfOpen},
			{name: "cb", from: circuitbreaker.StateHalfOpen, to: circuitbreaker.StateClosed},
		}, changes.all())
	})
}
