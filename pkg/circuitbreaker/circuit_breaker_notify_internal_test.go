package circuitbreaker

import (
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func alwaysReadyToTrip(Counts) bool {
	return true
}

// isLegalStateChange reports whether the state machine of the breaker has the change.
func isLegalStateChange(change pendingChange) bool {
	switch change {
	case pendingChange{from: StateClosed, to: StateOpen},
		pendingChange{from: StateOpen, to: StateHalfOpen},
		pendingChange{from: StateHalfOpen, to: StateOpen},
		pendingChange{from: StateHalfOpen, to: StateClosed}:
		return true
	default:
		return false
	}
}

// Run with -race: the callback appends to a slice that nothing guards, so the race
// detector fails the test when two callbacks of one breaker overlap.
func TestCircuitBreaker_OnStateChange_ConcurrentChangesAreDeliveredOneAtATimeInOrder(t *testing.T) {
	const (
		rounds = 200
		// A request changes the state at most twice, when it is let through (open to
		// half-open) and when it ends, so the requests of a round cannot overflow the
		// queue of pending state changes, which would drop some of them.
		requests = maxPendingStateChanges / 2
	)
	for round := range rounds {
		var delivered []pendingChange
		breaker := NewCircuitBreaker[bool](Settings{
			// The open period is over as soon as it starts: the requests keep the
			// breaker changing state.
			Timeout:     time.Nanosecond,
			ReadyToTrip: alwaysReadyToTrip,
			OnStateChange: func(_ string, from, to State) {
				delivered = append(delivered, pendingChange{from: from, to: to})
			},
		})
		start := make(chan struct{})
		var group sync.WaitGroup
		for request := range requests {
			group.Go(func() {
				<-start
				if request%2 == 0 {
					_ = fail(breaker)
				} else {
					_ = succeed(breaker)
				}
			})
		}
		close(start)
		group.Wait()

		// Every request has returned, so every change has been delivered. Reading the
		// state may cause a last change, delivered before State returns.
		final := breaker.State()
		previous := StateClosed
		for index, change := range delivered {
			require.Truef(t, isLegalStateChange(change), "round %d: change %d of %v is not a state change", round, index, delivered)
			require.Equalf(t, previous, change.from, "round %d: change %d of %v is out of order", round, index, delivered)
			previous = change.to
		}
		require.Equalf(t, final, previous, "round %d: the last of %v is not the final state", round, delivered)
	}
}

func TestCircuitBreaker_OnStateChange_DropsTheOldestQueuedChangesWhenTheQueueIsFull(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			openPeriod = 30 * time.Second
			dropped    = 3
		)
		var (
			delivered []pendingChange
			mutex     sync.Mutex
		)
		release := make(chan struct{})
		breaker := NewCircuitBreaker[bool](Settings{
			Timeout:     openPeriod,
			ReadyToTrip: alwaysReadyToTrip,
			OnStateChange: func(_ string, from, to State) {
				mutex.Lock()
				delivered = append(delivered, pendingChange{from: from, to: to})
				first := len(delivered) == 1
				mutex.Unlock()
				if first {
					<-release
				}
			},
		})
		tripped := make(chan error, 1)
		go func() { tripped <- fail(breaker) }()
		synctest.Wait()

		// The first callback does not return while the breaker half-opens and opens
		// again, over and over, until more changes have happened than the queue holds.
		queued := make([]pendingChange, 0, maxPendingStateChanges+dropped)
		for len(queued) < maxPendingStateChanges+dropped {
			time.Sleep(openPeriod + time.Second)
			require.Equal(t, StateHalfOpen, breaker.State())
			queued = append(queued, pendingChange{from: StateOpen, to: StateHalfOpen})
			require.NoError(t, fail(breaker))
			queued = append(queued, pendingChange{from: StateHalfOpen, to: StateOpen})
		}
		close(release)
		require.NoError(t, <-tripped)

		expected := append([]pendingChange{{from: StateClosed, to: StateOpen}}, queued[len(queued)-maxPendingStateChanges:]...)
		assert.Equal(t, StateOpen, breaker.State())
		mutex.Lock()
		defer mutex.Unlock()
		assert.Equal(t, expected, delivered)
	})
}
