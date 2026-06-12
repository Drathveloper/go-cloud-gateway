package gateway

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

// ErrCapture represents the error when the captured request body failed.
var ErrCapture = errors.New("capture body failed")

// ErrCaptureLimitExceeded represents the error when the body is larger than the capture limit.
var ErrCaptureLimitExceeded = errors.New("capture limit exceeded")

//nolint:gochecknoglobals
var bytesBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// copyBufSize matches the io.Copy internal buffer size.
const copyBufSize = 32 * 1024

//nolint:gochecknoglobals
var copyBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, copyBufSize)
		return &buf
	},
}

// Request represents a gateway request.
//
// The body of the request is read into memory and stored in the body field.
//
// The body field is nil if the original request body is empty.
type Request struct {
	URL        *url.URL
	Headers    http.Header
	BodyReader *ReplayableBody
	Method     string
	RemoteAddr string
}

// NewGatewayRequest creates a new gateway request from an http request.
func NewGatewayRequest(request *http.Request) *Request {
	return &Request{
		RemoteAddr: shared.GetRemoteAddr(request),
		URL:        request.URL,
		Method:     request.Method,
		Headers:    request.Header,
		BodyReader: NewReplayableBody(request.Body, request.ContentLength),
	}
}

// Response represents a gateway response.
//
// The body of the response is read into memory and stored in the body field.
//
// The body field is nil if the original response body is empty.
//
// The status field is the HTTP status code of the response.
type Response struct {
	Headers    http.Header
	BodyReader *ReplayableBody
	Status     int
}

// NewGatewayResponse creates a new gateway response from an http response.
func NewGatewayResponse(response *http.Response) *Response {
	return &Response{
		Status:     response.StatusCode,
		Headers:    response.Header,
		BodyReader: NewReplayableBody(response.Body, response.ContentLength),
	}
}

// ReplayableBody creates a new representation of the body that can be read multiple times.
type ReplayableBody struct {
	original io.ReadCloser
	reader   *bytes.Reader
	data     []byte
	length   int64
	captured bool
	closed   bool
}

// NewReplayableBody initializes a ReplayableBody allowing multiple reads from the same body by buffering the content.
func NewReplayableBody(original io.ReadCloser, length int64) *ReplayableBody {
	if original == nil {
		original = io.NopCloser(bytes.NewReader([]byte{}))
	}
	return &ReplayableBody{
		original: original,
		length:   length,
		captured: false,
	}
}

// Read reads data into the provided byte slice p and captures it into an internal buffer if not already captured.
// Returns the number of bytes read and any error encountered during the read operation.
func (rb *ReplayableBody) Read(output []byte) (int, error) {
	if rb.captured {
		n, err := rb.reader.Read(output)
		if errors.Is(err, io.EOF) {
			_, _ = rb.reader.Seek(0, io.SeekStart)
		}
		return n, err //nolint:wrapcheck
	}
	return rb.original.Read(output) //nolint:wrapcheck
}

// Capture reads the whole body content into an internal buffer, enabling it to be replayed
// multiple times, without a size limit. Prefer CaptureWithLimit when the body size is not
// otherwise bounded: an unlimited capture buffers attacker-sized bodies in memory.
func (rb *ReplayableBody) Capture() error {
	return rb.CaptureWithLimit(-1)
}

// CaptureWithLimit reads at most maxBytes of body content into an internal buffer, enabling
// it to be replayed multiple times. A negative maxBytes means no limit.
//
// When the body is larger than maxBytes it returns an error wrapping ErrCaptureLimitExceeded
// and the body is not captured, but it remains fully readable as a plain one-shot stream:
// the prefix consumed while probing is stitched back in front of the remaining data, so the
// request or response can still be forwarded.
func (rb *ReplayableBody) CaptureWithLimit(maxBytes int64) error {
	if rb.captured {
		return nil
	}
	if maxBytes >= 0 && rb.length > maxBytes {
		// The declared length already exceeds the limit: reject without consuming.
		return fmt.Errorf("%w: %w", ErrCapture, ErrCaptureLimitExceeded)
	}
	buf := bytesBufferPool.Get().(*bytes.Buffer) //nolint:forcetypeassert
	buf.Reset()
	defer bytesBufferPool.Put(buf)

	src := io.Reader(rb.original)
	if maxBytes >= 0 {
		// One extra byte to distinguish a body of exactly maxBytes from a larger one.
		src = io.LimitReader(rb.original, maxBytes+1)
	}
	length, err := io.Copy(buf, src)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrCapture, err.Error())
	}
	if maxBytes >= 0 && length > maxBytes {
		// Hand the consumed prefix back so the body remains forwardable.
		rb.original = newPrefixedReadCloser(bytes.Clone(buf.Bytes()), rb.original)
		return fmt.Errorf("%w: %w", ErrCapture, ErrCaptureLimitExceeded)
	}
	rb.length = length
	// The reader must own its bytes: buf returns to the pool and other
	// requests will overwrite its backing array.
	rb.data = bytes.Clone(buf.Bytes())
	rb.reader = bytes.NewReader(rb.data)
	rb.captured = true
	return nil
}

// Bytes returns the captured body content, or nil when the body has not been
// captured. The slice is the backing array of the replay reader: callers must
// treat it as read-only.
func (rb *ReplayableBody) Bytes() []byte {
	return rb.data
}

// WriteTo writes the remaining body to w. A captured body is written straight
// from the capture buffer and rewinds afterwards, mirroring the Read replay
// behavior; a non-captured body streams through a pooled buffer.
//
// Implementing io.WriterTo keeps io.Copy off the ResponseWriter ReadFrom
// fallback of net/http, which allocates a fresh 32KB buffer per response.
func (rb *ReplayableBody) WriteTo(writer io.Writer) (int64, error) {
	if rb.captured {
		written, err := rb.reader.WriteTo(writer)
		_, _ = rb.reader.Seek(0, io.SeekStart)
		return written, err //nolint:wrapcheck
	}
	bufPtr := copyBufPool.Get().(*[]byte) //nolint:forcetypeassert
	// The writer-only wrapper hides any ReaderFrom on w from io.CopyBuffer,
	// which would otherwise delegate to it and ignore the pooled buffer.
	written, err := io.CopyBuffer(writerOnly{writer}, rb.original, *bufPtr)
	copyBufPool.Put(bufPtr)
	return written, err //nolint:wrapcheck
}

// writerOnly hides every interface of the wrapped writer except io.Writer.
type writerOnly struct {
	io.Writer
}

// prefixedReadCloser replays an already-consumed prefix before the remaining body data.
type prefixedReadCloser struct {
	reader io.Reader
	closer io.Closer
}

func newPrefixedReadCloser(prefix []byte, rest io.ReadCloser) *prefixedReadCloser {
	return &prefixedReadCloser{
		reader: io.MultiReader(bytes.NewReader(prefix), rest),
		closer: rest,
	}
}

func (p *prefixedReadCloser) Read(output []byte) (int, error) {
	return p.reader.Read(output) //nolint:wrapcheck
}

func (p *prefixedReadCloser) Close() error {
	return p.closer.Close() //nolint:wrapcheck
}

// Len returns the body length.
func (rb *ReplayableBody) Len() int64 {
	return rb.length
}

// Close releases any resources associated with the ReplayableBody and closes the underlying source.
// It is idempotent: only the first call closes the underlying source.
// A captured body remains replayable after Close.
// Returns an error if any.
func (rb *ReplayableBody) Close() error {
	if rb.closed {
		return nil
	}
	rb.closed = true
	return rb.original.Close() //nolint:wrapcheck
}
