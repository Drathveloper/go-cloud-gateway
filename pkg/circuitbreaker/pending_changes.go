package circuitbreaker

// maxPendingStateChanges is how many state changes can wait for the OnStateChange
// callback. A breaker changes state at most a few times per open period, so the queue
// only fills up when the callback has stopped returning.
const maxPendingStateChanges = 16

// pendingChange is a state change the OnStateChange callback has not been called for yet.
type pendingChange struct {
	from State
	to   State
}

// pendingChanges holds the state changes that wait for the OnStateChange callback,
// oldest first. Its size is fixed: when it is full the oldest change makes room for the
// new one, so a callback that never returns costs changes, not memory, and the newest
// change, which says what the state is now, is never the one lost.
//
// The zero value is an empty queue. It is not safe for concurrent use.
type pendingChanges struct {
	changes [maxPendingStateChanges]pendingChange
	head    int
	size    int
}

func (q *pendingChanges) empty() bool {
	return q.size == 0
}

func (q *pendingChanges) push(change pendingChange) {
	if q.size == len(q.changes) {
		q.head = (q.head + 1) % len(q.changes)
		q.size--
	}
	q.changes[(q.head+q.size)%len(q.changes)] = change
	q.size++
}

func (q *pendingChanges) pop() (pendingChange, bool) {
	if q.size == 0 {
		return pendingChange{}, false
	}
	change := q.changes[q.head]
	q.head = (q.head + 1) % len(q.changes)
	q.size--
	return change, true
}
