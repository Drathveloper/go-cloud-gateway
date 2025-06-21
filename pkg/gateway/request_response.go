package gateway

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
)

// ErrCapture represents the error when the captured request body failed.
var ErrCapture = errors.New("capture body failed")

//nolint:gochecknoglobals
var bytesBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
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
		RemoteAddr: common.GetRemoteAddr(request),
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
	length   int64
	captured bool
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

// Capture reads the body content into an internal buffer, enabling it to be replayed multiple times.
func (rb *ReplayableBody) Capture() error {
	if rb.captured {
		return nil
	}
	buf := bytesBufferPool.Get().(*bytes.Buffer) //nolint:forcetypeassert
	buf.Reset()
	defer bytesBufferPool.Put(buf)

	length, err := io.Copy(buf, rb.original)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrCapture, err.Error())
	}
	rb.length = length
	rb.reader = bytes.NewReader(buf.Bytes())
	rb.captured = true
	return nil
}

// Len returns the body length.
func (rb *ReplayableBody) Len() int64 {
	return rb.length
}

// Close releases any resources associated with the ReplayableBody and closes the underlying source.
// Returns an error if any.
func (rb *ReplayableBody) Close() error {
	if !rb.captured {
		return rb.original.Close() //nolint:wrapcheck
	}
	return nil
}
