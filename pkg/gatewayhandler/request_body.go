package gatewayhandler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// unreadBodyGrace is how long a response waits for what is left of a request body that
// nobody is going to read: see requestBody.giveUpAfter.
const unreadBodyGrace = 100 * time.Millisecond

// errRequestBodyAbandoned is the error of the reads of a request body that was abandoned
// because the route timed out. It is a context.DeadlineExceeded: the http transport may
// report it instead of the timeout that caused it, and both must be answered alike.
var errRequestBodyAbandoned = fmt.Errorf("request body abandoned: %w", context.DeadlineExceeded)

// requestBody is the body of the inbound request. It sits under the replayable body of
// the gateway request, so every read of the client goes through it: the reads of the
// http transport that forwards the body to the backend, which run on another goroutine,
// and the ones of a filter that captures it.
//
// It knows whether the client has been read to the end, and it owns the read deadline of
// the connection in the two cases the server read timeout does not fit: see Abandon and
// renewReadDeadline.
//
// A nil requestBody is a request without a body: Abandon, renewReadDeadline and release
// are then no-ops.
type requestBody struct {
	body io.ReadCloser
	// writer is the response writer of the request, which gives access to the read
	// deadline of its connection.
	writer http.ResponseWriter
	// timeout is the deadline given to every read while renew is set.
	timeout time.Duration
	// renew is set while every read has to renew the read deadline.
	renew atomic.Bool
	// mu guards the fields below, and is held while the read deadline is set: no
	// deadline may be set once the body is done, as the connection moves on to
	// detecting a client that goes away and then to its next request.
	mu sync.Mutex
	// done is set once a read failed, io.EOF included, the body was closed or the
	// handler released it: nothing is left to read.
	done bool
	// abandoned is set once Abandon made the reads fail.
	abandoned bool
	// settled is set once the handler has decided how to answer: the body cannot be
	// abandoned after that. See settle.
	settled bool
	// givenUp is set once giveUpAfter has set a deadline for the rest of the body.
	givenUp bool
	// http1 is whether the request came over HTTP/1, where the server reads the rest
	// of the body before it writes the response.
	http1 bool
}

// trackRequestBody puts a requestBody in place of the body of the request and returns it.
// It returns nil when the request has no body, which HTTP/2 tells by its length alone.
func trackRequestBody(writer http.ResponseWriter, request *http.Request) *requestBody {
	if request.Body == nil || request.Body == http.NoBody || request.ContentLength == 0 {
		return nil
	}
	body := &requestBody{body: request.Body, writer: writer, http1: request.ProtoMajor == 1}
	request.Body = body
	return body
}

// Read reads from the client. After renewReadDeadline every read gets its own deadline.
func (b *requestBody) Read(output []byte) (int, error) {
	if b.renew.Load() {
		b.extendReadDeadline()
	}
	read, err := b.body.Read(output)
	if err != nil {
		err = b.finish(err)
	}
	return read, err
}

// Close closes the body of the request. The deadline of the last read is left in place:
// it bounds what net/http still reads of the body to be able to reuse the connection.
//
// The http transport closes the body before it has read it to the end when it gives up
// on the request, because the backend cannot be reached for instance. Closing a body
// makes net/http read what is left of it, so the transport, and the error it reports,
// would wait for the client: the rest of the body gets the same grace as in giveUpAfter.
// Once the handler is answering it has set that deadline itself, and a new one could
// outlive a body that the server has read to the end by then: see settle.
func (b *requestBody) Close() error {
	b.mu.Lock()
	answering := b.settled
	b.mu.Unlock()
	if !answering {
		b.giveUpAfter(unreadBodyGrace)
	}
	b.release()
	return b.body.Close() //nolint:wrapcheck
}

// Abandon makes the pending and the future reads of the body fail at once, unless it has
// been read to the end. The gateway calls it when the route times out and the handler
// before it answers a timeout: a body that is still arriving is of no use anymore, but
// the http transport does not return before the read it is waiting on does, and net/http
// reads what is left of the body before it writes the response, both for as long as the
// server read timeout allows. The server closes the connection after the response, as it
// cannot be reused without reading the rest of the body.
//
// It is safe to call from any goroutine and at any time: it does nothing once the
// handler has released the body.
func (b *requestBody) Abandon() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done || b.settled {
		return
	}
	b.abandoned = true
	_ = http.NewResponseController(b.writer).SetReadDeadline(time.Unix(1, 0))
}

// settle stops the body from being abandoned from now on and reports whether it has
// been. The handler calls it right before it answers, and closes the connection after
// the response of an abandoned body.
//
// net/http closes that connection on its own when part of the body is left unread, but
// not in the one case Abandon cannot tell apart: a read that is returning the end of the
// body. net/http has then already started the background read that detects a client that
// goes away, the deadline in the past makes that read fail, and net/http cancels the
// context of the connection for good, without closing it. Every later request of a
// connection kept alive in that state would start with a cancelled context.
func (b *requestBody) settle() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.settled = true
	return b.abandoned
}

// extendReadDeadline gives the read that is about to start its own deadline.
func (b *requestBody) extendReadDeadline() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.renew.Load() {
		_ = http.NewResponseController(b.writer).SetReadDeadline(time.Now().Add(b.timeout))
	}
}

// giveUpAfter bounds how long the server waits for what is left of the body, which the
// handler calls before it writes a response that does not need it: an error of the
// gateway, or the response of a backend that answered before the end of the body.
//
// On HTTP/1 the server reads the rest of the body before it writes the response, to be
// able to reuse the connection, for as long as the server read timeout allows: the
// client of a slow upload only learnt that it was rejected once it had sent everything.
// The response now waits for the given grace at most. A body that is complete by then,
// which is the usual case, keeps its connection: giving up at once would close the
// connection of every request with a body that the gateway rejects. Past the grace the
// response is sent and the server closes the connection.
//
// It does nothing on HTTP/2, where the server does not wait for the body, nor once the
// body has been read to the end or abandoned.
func (b *requestBody) giveUpAfter(grace time.Duration) {
	if b == nil || !b.http1 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done || b.abandoned {
		return
	}
	if err := http.NewResponseController(b.writer).SetReadDeadline(time.Now().Add(grace)); err == nil {
		// A read that is returning the end of the body clears it: see finish.
		b.givenUp = true
	}
}

// finish records that a read failed, so nothing is left to read, and returns the error
// the read must report.
//
// At the end of the body net/http clears the read deadline of the connection and starts
// the background read that detects a client that goes away. A deadline set after that
// makes that read fail, which cancels the request: renewReadDeadline and giveUpAfter may
// have set one while the last read was returning, so it is cleared here. After any other error the
// deadline stays: clearing it would let net/http wait for the client without a limit.
func (b *requestBody) finish(err error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.done = true
	renewed := b.renew.Swap(false)
	if errors.Is(err, io.EOF) {
		if renewed || b.givenUp {
			_ = http.NewResponseController(b.writer).SetReadDeadline(time.Time{})
		}
		return err
	}
	if b.abandoned {
		return fmt.Errorf("%w: %w", errRequestBodyAbandoned, err)
	}
	return err
}

// release stops the body from setting read deadlines. The handler calls it before it
// returns, as the connection is not its own after that, while the http transport may
// still be reading the body and the gateway abandoning it.
func (b *requestBody) release() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.done = true
	b.givenUp = false
	b.renew.Store(false)
}

// renewReadDeadline replaces the server read timeout, a deadline for the whole request,
// by a deadline of the given timeout renewed on every read. It is called when the
// response of a streaming route is about to be sent, and it does nothing when the body
// has been read to the end by then, which is the case unless the backend answers while
// the client is still sending.
//
// On HTTP/1 the server reads the rest of the body before it writes the response, and
// throws it away: the body can only keep flowing to the backend in full duplex mode,
// which is enabled here. HTTP/2 always allows it.
//
// A response writer that does not support read deadlines keeps the server read timeout.
func (b *requestBody) renewReadDeadline(timeout time.Duration) {
	if b == nil || timeout <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	// An abandoned body stays unreadable: its connection is closed after the response.
	if b.done || b.abandoned {
		return
	}
	controller := http.NewResponseController(b.writer)
	// This also bounds a read that is already waiting for the client.
	if err := controller.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return
	}
	_ = controller.EnableFullDuplex()
	b.timeout = timeout
	b.renew.Store(true)
}
