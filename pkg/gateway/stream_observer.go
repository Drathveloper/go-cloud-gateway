package gateway

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

// ErrStreamTruncated is delivered to a stream observer's onDone callback when the
// body is closed before its stream reached EOF: the client disconnected, an error
// handler discarded the response, or a filter aborted the pipeline after the
// backend had already replied.
var ErrStreamTruncated = errors.New("body stream closed before EOF")

// ErrObserverPanic is delivered to a stream observer's onDone callback when its own
// onChunk callback panicked. The stream itself is never affected: observation must
// not break delivery, and request-side callbacks run on transport goroutines where
// an unrecovered panic would crash the whole process.
var ErrObserverPanic = errors.New("stream observer callback panicked")

// ObserveStream registers callbacks invoked as the body streams from its source,
// without buffering it. It is the streaming-friendly alternative to Capture for
// filters that only need to watch the bytes flow by (metering token usage, counting
// server-sent events, measuring stream size and duration) rather than replay them.
//
// onChunk is called with each chunk in source-read order. The slice is only valid
// for the duration of the call: it aliases the caller's read buffer and is reused,
// so copy anything that must outlive the callback. Chunks are transport-sized reads,
// NOT application messages: a server-sent-event consumer must reassemble events from
// the chunk stream itself.
//
// onDone fires exactly once per registration: a nil error on a clean EOF, the read
// error when the source fails mid-stream, or ErrStreamTruncated when the body is
// closed before EOF. totalBytes is the number of bytes observed before completion.
//
// Either callback may be nil. Passing both nil is a no-op.
//
// A body that is already captured, or declared empty (length 0), completes
// synchronously: onChunk (when non-nil and the captured body is non-empty) and onDone
// are invoked before ObserveStream returns. Observers see source bytes once, at the
// moment they are first read from the backend or client; replays of a captured body
// to the client never re-fire. Registering several observers nests them, and each
// sees every byte that flows out of the source after its own registration exactly once.
//
// Concurrency: response-body callbacks run on the handler goroutine while the gateway
// Context is still valid. Request-body callbacks may run on a transport goroutine, per
// the http.RoundTripper contract — they must be safe for that and must not retain the
// *Context. ObserveStream itself must not be called concurrently with reads of the body.
func (rb *ReplayableBody) ObserveStream(onChunk func(chunk []byte), onDone func(totalBytes int64, err error)) {
	if onChunk == nil && onDone == nil {
		return
	}
	// Already-buffered or trivially-empty bodies have no live source to wrap: their
	// bytes (if any) were observed when first read, so completion is reported now.
	// The captured branch mirrors a replay so filter code is identical regardless of
	// whether an earlier filter captured the body.
	if rb.captured {
		fireSyncReplay(onChunk, onDone, rb.data)
		return
	}
	// A declared-empty body is the same signal buildProxyRequest trusts to drop the
	// request body entirely (req.Body = nil): the source is never read, so no chunk
	// will ever flow and completion is immediate.
	if rb.length == 0 {
		fireSync(onDone, 0, nil)
		return
	}
	// The source has already been closed without being drained: report truncation
	// rather than leaving onDone forever pending.
	if rb.closed {
		fireSync(onDone, 0, ErrStreamTruncated)
		return
	}
	// Wrap the CURRENT source, not a shared observer list: a failed CaptureWithLimit
	// stitches a prefixedReadCloser in front of the source, and a late observer must
	// wrap that stitched reader to see the probed prefix exactly once. Nesting keeps
	// the invariant trivially correct — each observer sees every byte emitted by the
	// source after its registration, once.
	rb.original = &observedBody{
		inner:   rb.original,
		onChunk: onChunk,
		onDone:  onDone,
	}
}

// fireSyncReplay reports a synchronous, already-buffered body to a freshly registered
// observer: the chunk (when present) followed by a clean completion.
func fireSyncReplay(onChunk func([]byte), onDone func(int64, error), data []byte) {
	if onChunk != nil && len(data) > 0 {
		onChunk(data)
	}
	fireSync(onDone, int64(len(data)), nil)
}

// fireSync invokes onDone when it is non-nil. It runs in the caller's stack (still
// under the handler's panic recovery for filter code), so callback panics propagate
// with normal filter-panic semantics rather than being swallowed here.
func fireSync(onDone func(int64, error), totalBytes int64, err error) {
	if onDone != nil {
		onDone(totalBytes, err)
	}
}

// observedBody wraps a body source and fires observer callbacks as it is read.
//
// It deliberately implements only Read and Close. A WriterTo or ReaderFrom would let
// io.Copy in the handler bypass the per-chunk Read calls that observation depends on,
// the same hazard writerOnly guards against on the write side.
type observedBody struct {
	inner         io.ReadCloser
	onChunk       func([]byte)
	onDone        func(int64, error)
	mu            sync.Mutex
	n             int64
	done          bool
	chunkDisabled bool
}

// Read reads from the inner source and reports the bytes to the observer.
//
// inner.Read runs outside the lock: holding it across a network read would make a
// concurrent Close (client disconnect) block until the read returns. Only callback
// firing and the state transition are serialized, which is enough to guarantee
// exactly-once onDone and no onChunk after onDone.
func (ob *observedBody) Read(output []byte) (int, error) {
	read, err := ob.inner.Read(output)

	ob.mu.Lock()
	defer ob.mu.Unlock()
	if ob.done {
		return read, err //nolint:wrapcheck
	}
	if read > 0 {
		ob.n += int64(read)
		ob.fireChunk(output[:read])
	}
	// !ob.done also covers the case where fireChunk just completed the observer with
	// ErrObserverPanic: a read returning data together with io.EOF must not fire twice.
	if err != nil && !ob.done {
		ob.done = true
		// A clean end of stream is not an observer error.
		if errors.Is(err, io.EOF) {
			ob.fireDone(nil)
		} else {
			ob.fireDone(err)
		}
	}
	return read, err //nolint:wrapcheck
}

// Close closes the inner source. If the stream had not reached completion yet, the
// observer is told the stream was truncated. It is safe to call concurrently with Read
// and more than once: only the first completion fires onDone.
func (ob *observedBody) Close() error {
	ob.mu.Lock()
	if !ob.done {
		ob.done = true
		ob.fireDone(ErrStreamTruncated)
	}
	ob.mu.Unlock()
	return ob.inner.Close() //nolint:wrapcheck
}

// fireChunk delivers a chunk to onChunk under recovery. A panicking onChunk would
// otherwise reach net/http's connection serve loop (response side, past the handler's
// recover) or crash the process (request side, on a transport goroutine). A panic
// disables this observer's onChunk for the rest of the stream and surfaces as an
// ErrObserverPanic completion so the filter author sees the bug instead of silence.
// The caller holds ob.mu.
func (ob *observedBody) fireChunk(chunk []byte) {
	if ob.onChunk == nil || ob.chunkDisabled {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			ob.chunkDisabled = true
			ob.done = true
			ob.fireDone(fmt.Errorf("%w: %v", ErrObserverPanic, recovered))
		}
	}()
	ob.onChunk(chunk)
}

// fireDone delivers completion to onDone under recovery: a panicking onDone must never
// escape onto a transport goroutine or the handler copy loop. The caller holds ob.mu.
func (ob *observedBody) fireDone(err error) {
	if ob.onDone == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	ob.onDone(ob.n, err)
}
