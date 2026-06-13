package gateway_test

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// scriptedRead is a single programmed result of a Read call.
type scriptedRead struct {
	err  error
	data []byte
}

// scriptedReader returns a fixed sequence of reads. Each entry yields its bytes and its
// error in the SAME Read call, so it can produce the (n>0, io.EOF) and (n>0, err) shapes
// that a chain of bytes.Readers cannot. It also counts Close calls.
type scriptedReader struct {
	closeErr error
	reads    []scriptedRead
	idx      int
	closes   int
}

func (s *scriptedReader) Read(output []byte) (int, error) {
	if s.idx >= len(s.reads) {
		return 0, io.EOF
	}
	read := s.reads[s.idx]
	s.idx++
	n := copy(output, read.data)
	if n < len(read.data) {
		panic("scriptedReader: read buffer too small for scripted chunk")
	}
	return n, read.err
}

func (s *scriptedReader) Close() error {
	s.closes++
	return s.closeErr
}

// chunkRecorder records observed chunks (copied, since the slice is only valid during the
// call) and the completion outcome. Not safe for concurrent use; the race test uses its
// own atomic counter instead.
type chunkRecorder struct {
	err       error
	chunks    [][]byte
	total     int64
	doneCalls int
}

func (r *chunkRecorder) onChunk(chunk []byte) {
	r.chunks = append(r.chunks, bytes.Clone(chunk))
}

func (r *chunkRecorder) onDone(total int64, err error) {
	r.doneCalls++
	r.total = total
	r.err = err
}

func (r *chunkRecorder) joined() []byte {
	return bytes.Join(r.chunks, nil)
}

func TestReplayableBody_ObserveStream_WriteToPath(t *testing.T) {
	payload := []byte("hello streaming world")
	src := &scriptedReader{reads: []scriptedRead{
		{data: []byte("hello ")},
		{data: []byte("streaming ")},
		{data: []byte("world")},
	}}
	rb := gateway.NewReplayableBody(src, int64(len(payload)))

	var rec chunkRecorder
	rb.ObserveStream(rec.onChunk, rec.onDone)

	var sink bytes.Buffer
	if _, err := io.Copy(&sink, rb); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if !bytes.Equal(sink.Bytes(), payload) {
		t.Errorf("client got %q, want %q", sink.Bytes(), payload)
	}
	if !bytes.Equal(rec.joined(), payload) {
		t.Errorf("observed %q, want %q", rec.joined(), payload)
	}
	if rec.doneCalls != 1 || rec.err != nil || rec.total != int64(len(payload)) {
		t.Errorf("onDone calls=%d err=%v total=%d, want 1 calls, nil err, total %d",
			rec.doneCalls, rec.err, rec.total, len(payload))
	}
}

func TestReplayableBody_ObserveStream_ReadPath(t *testing.T) {
	payload := []byte("read path bytes")
	src := &scriptedReader{reads: []scriptedRead{
		{data: []byte("read ")},
		{data: []byte("path ")},
		{data: []byte("bytes")},
	}}
	rb := gateway.NewReplayableBody(src, int64(len(payload)))

	var rec chunkRecorder
	rb.ObserveStream(rec.onChunk, rec.onDone)

	got, err := io.ReadAll(rb)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !bytes.Equal(got, payload) || !bytes.Equal(rec.joined(), payload) {
		t.Errorf("read %q observed %q, want %q", got, rec.joined(), payload)
	}
	if rec.doneCalls != 1 || rec.err != nil {
		t.Errorf("onDone calls=%d err=%v, want 1 calls and nil err", rec.doneCalls, rec.err)
	}
}

func TestReplayableBody_ObserveStream_DataWithEOFInSameRead(t *testing.T) {
	src := &scriptedReader{reads: []scriptedRead{
		{data: []byte("first")},
		{data: []byte("last"), err: io.EOF},
	}}
	rb := gateway.NewReplayableBody(src, -1)

	var rec chunkRecorder
	rb.ObserveStream(rec.onChunk, rec.onDone)

	if _, err := io.Copy(io.Discard, rb); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if !bytes.Equal(rec.joined(), []byte("firstlast")) {
		t.Errorf("observed %q, want %q", rec.joined(), "firstlast")
	}
	if rec.doneCalls != 1 || rec.err != nil {
		t.Errorf("onDone calls=%d err=%v, want 1 calls and nil err", rec.doneCalls, rec.err)
	}
}

func TestReplayableBody_ObserveStream_MidStreamReadError(t *testing.T) {
	boom := errors.New("backend exploded")
	src := &scriptedReader{reads: []scriptedRead{
		{data: []byte("partial")},
		{data: []byte("more"), err: boom},
	}}
	rb := gateway.NewReplayableBody(src, -1)

	var rec chunkRecorder
	rb.ObserveStream(rec.onChunk, rec.onDone)

	_, _ = io.Copy(io.Discard, rb)
	if rec.doneCalls != 1 || !errors.Is(rec.err, boom) {
		t.Errorf("onDone calls=%d err=%v, want 1 calls and the read error", rec.doneCalls, rec.err)
	}
	// "more" arrives with the error and must still be observed before completion.
	if !bytes.Equal(rec.joined(), []byte("partialmore")) {
		t.Errorf("observed %q, want %q", rec.joined(), "partialmore")
	}
}

func TestReplayableBody_ObserveStream_CloseBeforeEOF(t *testing.T) {
	src := &scriptedReader{reads: []scriptedRead{
		{data: []byte("abc")},
		{data: []byte("def")},
		{data: []byte("ghi")},
	}}
	rb := gateway.NewReplayableBody(src, -1)

	var rec chunkRecorder
	rb.ObserveStream(rec.onChunk, rec.onDone)

	buf := make([]byte, 3)
	if _, err := rb.Read(buf); err != nil { // observes "abc", stream not done
		t.Fatalf("read failed: %v", err)
	}
	if err := rb.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if rec.doneCalls != 1 || !errors.Is(rec.err, gateway.ErrStreamTruncated) {
		t.Errorf("onDone calls=%d err=%v, want 1 calls and ErrStreamTruncated", rec.doneCalls, rec.err)
	}
	if rec.total != 3 {
		t.Errorf("observed total=%d, want 3", rec.total)
	}
	if src.closes != 1 {
		t.Errorf("inner closes=%d, want 1", src.closes)
	}
}

func TestReplayableBody_ObserveStream_CloseAfterEOFFiresOnce(t *testing.T) {
	src := &scriptedReader{reads: []scriptedRead{{data: []byte("done")}}}
	rb := gateway.NewReplayableBody(src, -1)

	var rec chunkRecorder
	rb.ObserveStream(rec.onChunk, rec.onDone)

	if _, err := io.Copy(io.Discard, rb); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	_ = rb.Close()
	_ = rb.Close() // idempotent
	if rec.doneCalls != 1 || rec.err != nil {
		t.Errorf("onDone calls=%d err=%v, want exactly 1 clean completion", rec.doneCalls, rec.err)
	}
	if src.closes != 1 {
		t.Errorf("inner closes=%d, want 1 (ReplayableBody.Close idempotent)", src.closes)
	}
}

func TestReplayableBody_ObserveStream_TwoObserversNested(t *testing.T) {
	payload := []byte("shared body")
	src := &scriptedReader{reads: []scriptedRead{
		{data: []byte("shared")},
		{data: []byte(" body")},
	}}
	rb := gateway.NewReplayableBody(src, int64(len(payload)))

	var order []string
	var first, second chunkRecorder
	rb.ObserveStream(func(c []byte) { order = append(order, "first"); first.onChunk(c) }, first.onDone)
	rb.ObserveStream(func(c []byte) { order = append(order, "second"); second.onChunk(c) }, second.onDone)

	if _, err := io.Copy(io.Discard, rb); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if !bytes.Equal(first.joined(), payload) || !bytes.Equal(second.joined(), payload) {
		t.Errorf("observers saw %q / %q, want %q", first.joined(), second.joined(), payload)
	}
	if first.doneCalls != 1 || second.doneCalls != 1 {
		t.Errorf("done calls first=%d second=%d, want 1 each", first.doneCalls, second.doneCalls)
	}
	// Registration order: the first registered observer wraps the source innermost, so
	// it sees each chunk first.
	if len(order) < 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("callback order=%v, want first before second per chunk", order)
	}
}

func TestReplayableBody_ObserveStream_AfterCapture(t *testing.T) {
	payload := []byte("captured payload")
	rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(payload)), int64(len(payload)))
	if err := rb.Capture(); err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	var rec chunkRecorder
	rb.ObserveStream(rec.onChunk, rec.onDone) // must complete synchronously

	if rec.doneCalls != 1 || rec.err != nil || !bytes.Equal(rec.joined(), payload) {
		t.Errorf("sync replay: calls=%d err=%v observed=%q, want 1 clean call observing %q",
			rec.doneCalls, rec.err, rec.joined(), payload)
	}
	// Replays to the client must not re-fire observers.
	for range 2 {
		var sink bytes.Buffer
		if _, err := rb.WriteTo(&sink); err != nil {
			t.Fatalf("writeTo failed: %v", err)
		}
		if !bytes.Equal(sink.Bytes(), payload) {
			t.Errorf("replay got %q, want %q", sink.Bytes(), payload)
		}
	}
	if rec.doneCalls != 1 {
		t.Errorf("onDone fired %d times across replays, want 1", rec.doneCalls)
	}
}

func TestReplayableBody_ObserveStream_CaptureAfterObserve(t *testing.T) {
	payload := []byte("observe then capture")
	rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(payload)), int64(len(payload)))

	var rec chunkRecorder
	rb.ObserveStream(rec.onChunk, rec.onDone)

	if err := rb.Capture(); err != nil { // observers fire here, reading the source
		t.Fatalf("capture failed: %v", err)
	}
	if rec.doneCalls != 1 || rec.err != nil || !bytes.Equal(rec.joined(), payload) {
		t.Errorf("capture-time observation: calls=%d err=%v observed=%q, want 1 clean call observing %q",
			rec.doneCalls, rec.err, rec.joined(), payload)
	}
	// Replays via Read and WriteTo must not re-fire.
	if _, err := io.ReadAll(rb); err != nil {
		t.Fatalf("read replay failed: %v", err)
	}
	var sink bytes.Buffer
	if _, err := rb.WriteTo(&sink); err != nil {
		t.Fatalf("writeTo replay failed: %v", err)
	}
	if rec.doneCalls != 1 {
		t.Errorf("onDone fired %d times, want 1 (replays must not re-fire)", rec.doneCalls)
	}
}

func TestReplayableBody_ObserveStream_ObserveThenCaptureWithLimitExceeded(t *testing.T) {
	payload := []byte("0123456789")
	rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(payload)), -1)

	var rec chunkRecorder
	rb.ObserveStream(rec.onChunk, rec.onDone)

	if err := rb.CaptureWithLimit(4); !errors.Is(err, gateway.ErrCaptureLimitExceeded) {
		t.Fatalf("expected ErrCaptureLimitExceeded, actual %v", err)
	}
	// The failed capture probes a prefix but the stream is not done: onDone must wait.
	if rec.doneCalls != 0 {
		t.Errorf("onDone fired %d times after failed capture, want 0", rec.doneCalls)
	}
	got, err := io.ReadAll(rb)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("body not fully forwardable: got %q err %v", got, err)
	}
	if !bytes.Equal(rec.joined(), payload) {
		t.Errorf("observed %q, want %q (each byte exactly once)", rec.joined(), payload)
	}
	if rec.doneCalls != 1 || rec.err != nil || rec.total != int64(len(payload)) {
		t.Errorf("onDone calls=%d err=%v total=%d, want 1 clean completion of %d bytes",
			rec.doneCalls, rec.err, rec.total, len(payload))
	}
}

func TestReplayableBody_ObserveStream_CaptureWithLimitExceededThenObserve(t *testing.T) {
	payload := []byte("0123456789")
	rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(payload)), -1)

	if err := rb.CaptureWithLimit(4); !errors.Is(err, gateway.ErrCaptureLimitExceeded) {
		t.Fatalf("expected ErrCaptureLimitExceeded, actual %v", err)
	}

	var rec chunkRecorder
	rb.ObserveStream(rec.onChunk, rec.onDone) // wraps the stitched prefixedReadCloser

	got, err := io.ReadAll(rb)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("body not fully forwardable: got %q err %v", got, err)
	}
	if !bytes.Equal(rec.joined(), payload) {
		t.Errorf("late observer saw %q, want %q (prefix + rest, each byte once)", rec.joined(), payload)
	}
	if rec.doneCalls != 1 || rec.err != nil {
		t.Errorf("onDone calls=%d err=%v, want 1 clean completion", rec.doneCalls, rec.err)
	}
}

func TestReplayableBody_ObserveStream_EmptyAndClosedBodies(t *testing.T) {
	t.Run("nil source length zero completes immediately", func(t *testing.T) {
		rb := gateway.NewReplayableBody(nil, 0)
		var rec chunkRecorder
		rb.ObserveStream(rec.onChunk, rec.onDone)
		if rec.doneCalls != 1 || rec.err != nil || rec.total != 0 || len(rec.chunks) != 0 {
			t.Errorf("calls=%d err=%v total=%d chunks=%d, want 1 clean call, no chunks",
				rec.doneCalls, rec.err, rec.total, len(rec.chunks))
		}
	})

	t.Run("captured empty body completes immediately without chunk", func(t *testing.T) {
		rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(nil)), 0)
		if err := rb.Capture(); err != nil {
			t.Fatalf("capture failed: %v", err)
		}
		var rec chunkRecorder
		rb.ObserveStream(rec.onChunk, rec.onDone)
		if rec.doneCalls != 1 || rec.err != nil || len(rec.chunks) != 0 {
			t.Errorf("calls=%d err=%v chunks=%d, want 1 clean call and no chunk for empty body",
				rec.doneCalls, rec.err, len(rec.chunks))
		}
	})

	t.Run("closed uncaptured body reports truncation", func(t *testing.T) {
		rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader([]byte("data"))), -1)
		if err := rb.Close(); err != nil {
			t.Fatalf("close failed: %v", err)
		}
		var rec chunkRecorder
		rb.ObserveStream(rec.onChunk, rec.onDone)
		if rec.doneCalls != 1 || !errors.Is(rec.err, gateway.ErrStreamTruncated) {
			t.Errorf("calls=%d err=%v, want 1 call with ErrStreamTruncated", rec.doneCalls, rec.err)
		}
	})
}

func TestReplayableBody_ObserveStream_NilCallbacks(t *testing.T) {
	payload := []byte("payload")

	t.Run("both nil leaves the body unchanged", func(t *testing.T) {
		rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(payload)), int64(len(payload)))
		rb.ObserveStream(nil, nil)
		got, err := io.ReadAll(rb)
		if err != nil || !bytes.Equal(got, payload) {
			t.Errorf("read %q err %v, want %q", got, err, payload)
		}
	})

	t.Run("only onDone", func(t *testing.T) {
		rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(payload)), int64(len(payload)))
		var total int64
		var calls int
		rb.ObserveStream(nil, func(n int64, _ error) { calls++; total = n })
		_, _ = io.Copy(io.Discard, rb)
		if calls != 1 || total != int64(len(payload)) {
			t.Errorf("calls=%d total=%d, want 1 call total %d", calls, total, len(payload))
		}
	})

	t.Run("only onChunk", func(t *testing.T) {
		rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(payload)), int64(len(payload)))
		var seen []byte
		rb.ObserveStream(func(c []byte) { seen = append(seen, c...) }, nil)
		_, _ = io.Copy(io.Discard, rb)
		if !bytes.Equal(seen, payload) {
			t.Errorf("observed %q, want %q", seen, payload)
		}
	})
}

func TestReplayableBody_ObserveStream_OnChunkPanic(t *testing.T) {
	payload := []byte("panic payload here")
	src := &scriptedReader{reads: []scriptedRead{
		{data: []byte("panic ")},
		{data: []byte("payload ")},
		{data: []byte("here")},
	}}
	rb := gateway.NewReplayableBody(src, int64(len(payload)))

	var panicRec chunkRecorder
	var goodRec chunkRecorder
	rb.ObserveStream(func([]byte) { panic("observer blew up") }, panicRec.onDone)
	rb.ObserveStream(goodRec.onChunk, goodRec.onDone)

	var sink bytes.Buffer
	if _, err := io.Copy(&sink, rb); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	// Delivery to the client is never affected by an observer panic.
	if !bytes.Equal(sink.Bytes(), payload) {
		t.Errorf("client got %q, want %q", sink.Bytes(), payload)
	}
	// The panicking observer is completed with ErrObserverPanic.
	if panicRec.doneCalls != 1 || !errors.Is(panicRec.err, gateway.ErrObserverPanic) {
		t.Errorf("panic observer calls=%d err=%v, want 1 call with ErrObserverPanic",
			panicRec.doneCalls, panicRec.err)
	}
	// A well-behaved observer registered alongside it is unaffected.
	if !bytes.Equal(goodRec.joined(), payload) || goodRec.doneCalls != 1 || goodRec.err != nil {
		t.Errorf("good observer observed=%q calls=%d err=%v, want full body and 1 clean call",
			goodRec.joined(), goodRec.doneCalls, goodRec.err)
	}
}

func TestReplayableBody_ObserveStream_OnDonePanicRecovered(t *testing.T) {
	payload := []byte("done panic")
	src := &scriptedReader{reads: []scriptedRead{{data: payload}}}
	rb := gateway.NewReplayableBody(src, int64(len(payload)))

	rb.ObserveStream(nil, func(int64, error) { panic("done blew up") })

	var sink bytes.Buffer
	if _, err := io.Copy(&sink, rb); err != nil {
		t.Fatalf("copy must complete despite onDone panic: %v", err)
	}
	if !bytes.Equal(sink.Bytes(), payload) {
		t.Errorf("client got %q, want %q", sink.Bytes(), payload)
	}
}

func TestReplayableBody_ObserveStream_LenUnaffected(t *testing.T) {
	tests := []struct {
		name   string
		length int64
	}{
		{name: "empty", length: 0},
		{name: "unknown", length: -1},
		{name: "known", length: 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader([]byte("hello world"))), tt.length)
			before := rb.Len()
			rb.ObserveStream(func([]byte) {}, func(int64, error) {})
			if rb.Len() != before {
				t.Errorf("Len changed from %d to %d after ObserveStream", before, rb.Len())
			}
		})
	}
}

func TestReplayableBody_ObserveStream_ConcurrentReadClose(t *testing.T) {
	reader, writer := io.Pipe()
	rb := gateway.NewReplayableBody(reader, -1)

	var doneCalls atomic.Int32
	rb.ObserveStream(nil, func(int64, error) { doneCalls.Add(1) })

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = writer.Write([]byte("streaming"))
		// Leave the pipe open so the reader blocks, then race a Close against it.
		_ = rb.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(io.Discard, rb)
	}()
	wg.Wait()

	if got := doneCalls.Load(); got != 1 {
		t.Errorf("onDone fired %d times under concurrent Read/Close, want exactly 1", got)
	}
	_ = writer.Close()
}
