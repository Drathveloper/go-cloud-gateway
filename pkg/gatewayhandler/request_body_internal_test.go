package gatewayhandler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

var errClientGone = errors.New("client gone")

// connRecorder is a response writer that records what a request body asks of the
// connection of its request.
type connRecorder struct {
	http.ResponseWriter

	// deadlineErr is the error SetReadDeadline answers with.
	deadlineErr   error
	readDeadlines []time.Time
	fullDuplex    int
	mu            sync.Mutex
}

func newConnRecorder() *connRecorder {
	return &connRecorder{ResponseWriter: httptest.NewRecorder()}
}

func (r *connRecorder) SetReadDeadline(deadline time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.readDeadlines = append(r.readDeadlines, deadline)
	return r.deadlineErr
}

func (r *connRecorder) EnableFullDuplex() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fullDuplex++
	return nil
}

func (r *connRecorder) deadlines() []time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Time(nil), r.readDeadlines...)
}

func (r *connRecorder) fullDuplexCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fullDuplex
}

// scriptedBody hands out its chunks one per read and then fails with err, or with io.EOF
// when there is none. With endWithData the error comes along with the last chunk, which
// is how net/http ends a body of known length.
type scriptedBody struct {
	err         error
	chunks      []string
	closes      int
	endWithData bool
}

func (b *scriptedBody) Read(output []byte) (int, error) {
	end := b.err
	if end == nil {
		end = io.EOF
	}
	if len(b.chunks) == 0 {
		return 0, end
	}
	read := copy(output, b.chunks[0])
	if read < len(b.chunks[0]) {
		b.chunks[0] = b.chunks[0][read:]
		return read, nil
	}
	b.chunks = b.chunks[1:]
	if len(b.chunks) == 0 && b.endWithData {
		return read, end
	}
	return read, nil
}

func (b *scriptedBody) Close() error {
	b.closes++
	return nil
}

func newTrackedBody(tb testing.TB, source io.ReadCloser, writer http.ResponseWriter) *requestBody {
	tb.Helper()
	request := httptest.NewRequestWithContext(tb.Context(), http.MethodPost, "http://localhost/upload", source)
	request.ContentLength = -1
	body := trackRequestBody(writer, request)
	require.NotNil(tb, body)
	return body
}

func TestTrackRequestBody(t *testing.T) {
	tests := []struct {
		body          io.ReadCloser
		name          string
		contentLength int64
		expectTracked bool
	}{
		{name: "request without a body should not be tracked", body: nil},
		{name: "request with no body should not be tracked", body: http.NoBody},
		{
			name:          "request with a body of no length should not be tracked",
			body:          io.NopCloser(strings.NewReader("")),
			contentLength: 0,
		},
		{
			name:          "request with a body of known length should be tracked",
			body:          io.NopCloser(strings.NewReader("payload")),
			contentLength: 7,
			expectTracked: true,
		},
		{
			name:          "request with a body of unknown length should be tracked",
			body:          io.NopCloser(strings.NewReader("payload")),
			contentLength: -1,
			expectTracked: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost/upload", nil)
			request.Body = tt.body
			request.ContentLength = tt.contentLength

			body := trackRequestBody(httptest.NewRecorder(), request)

			if !tt.expectTracked {
				assert.Nil(t, body)
				assert.Equal(t, tt.body, request.Body)
				return
			}
			require.NotNil(t, body)
			assert.Same(t, body, request.Body)
			content, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			assert.Equal(t, "payload", string(content))
		})
	}
}

// The tracked body sits under the replayable body of the gateway request, which must
// keep working as it does on the bare body.
func TestRequestBody_KeepsTheReplayableBody(t *testing.T) {
	const payload = "a request body to forward"
	newRequest := func(t *testing.T, length int64) (*gateway.Request, *scriptedBody) {
		t.Helper()
		source := &scriptedBody{chunks: []string{"a request ", "body ", "to forward"}}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost/upload", source)
		request.ContentLength = length
		require.NotNil(t, trackRequestBody(httptest.NewRecorder(), request))
		return gateway.NewGatewayRequest(request), source
	}

	t.Run("length should be the one of the request", func(t *testing.T) {
		request, _ := newRequest(t, int64(len(payload)))

		assert.Equal(t, int64(len(payload)), request.BodyReader.Len())
	})
	t.Run("body should stream to a writer", func(t *testing.T) {
		request, _ := newRequest(t, int64(len(payload)))
		var forwarded bytes.Buffer

		written, err := request.BodyReader.WriteTo(&forwarded)

		require.NoError(t, err)
		assert.Equal(t, int64(len(payload)), written)
		assert.Equal(t, payload, forwarded.String())
	})
	t.Run("captured body should be replayed", func(t *testing.T) {
		request, _ := newRequest(t, -1)

		require.NoError(t, request.BodyReader.Capture())

		assert.Equal(t, int64(len(payload)), request.BodyReader.Len())

		assert.Equal(t, payload, string(request.BodyReader.Bytes()))
		for range 2 {
			replayed, err := io.ReadAll(request.BodyReader)
			require.NoError(t, err)
			assert.Equal(t, payload, string(replayed))
		}
	})
	t.Run("body over the capture limit should still be forwarded", func(t *testing.T) {
		request, _ := newRequest(t, -1)

		err := request.BodyReader.CaptureWithLimit(4)

		require.ErrorIs(t, err, gateway.ErrCaptureLimitExceeded)
		forwarded, err := io.ReadAll(request.BodyReader)
		require.NoError(t, err)
		assert.Equal(t, payload, string(forwarded))
	})
	t.Run("close should close the body once", func(t *testing.T) {
		request, source := newRequest(t, int64(len(payload)))

		require.NoError(t, request.BodyReader.Close())
		require.NoError(t, request.BodyReader.Close())

		assert.Equal(t, 1, source.closes)
	})
}

func TestRequestBody_Read_LeavesTheServerReadTimeoutAlone(t *testing.T) {
	recorder := newConnRecorder()
	body := newTrackedBody(t, &scriptedBody{chunks: []string{"one", "two"}}, recorder)

	content, err := io.ReadAll(body)

	require.NoError(t, err)
	assert.Equal(t, "onetwo", string(content))
	assert.Empty(t, recorder.deadlines())
	assert.Zero(t, recorder.fullDuplexCalls())
}

func TestRequestBody_RenewReadDeadline(t *testing.T) {
	tests := []struct {
		name        string
		endWithData bool
	}{
		{name: "body that ends after its last chunk"},
		{name: "body that ends with its last chunk", endWithData: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const timeout = time.Minute
				recorder := newConnRecorder()
				source := &scriptedBody{chunks: []string{"one", "two", "three"}, endWithData: tt.endWithData}
				body := newTrackedBody(t, source, recorder)
				buf := make([]byte, 16)
				start := time.Now()

				// A read that is already waiting gets the deadline as well.
				body.renewReadDeadline(timeout)
				expected := make([]time.Time, 0, 3)
				expected = append(expected, start.Add(timeout))
				assert.Equal(t, expected, recorder.deadlines())
				assert.Equal(t, 1, recorder.fullDuplexCalls())

				// Every read has its own deadline, however long the previous ones took.
				for range 2 {
					time.Sleep(10 * time.Minute)
					_, err := body.Read(buf)
					require.NoError(t, err)
					expected = append(expected, time.Now().Add(timeout))
					assert.Equal(t, expected, recorder.deadlines())
				}

				// The end of the body takes the deadline off the connection, and
				// nothing sets one after that.
				_, err := io.ReadAll(body)
				require.NoError(t, err)
				deadlines := recorder.deadlines()
				assert.True(t, deadlines[len(deadlines)-1].IsZero(), "expected the deadline to be cleared")
				_, err = body.Read(buf)
				require.ErrorIs(t, err, io.EOF)
				body.renewReadDeadline(timeout)
				assert.Equal(t, deadlines, recorder.deadlines())
				assert.Equal(t, 1, recorder.fullDuplexCalls())
			})
		})
	}
}

// After a failed read net/http may still read from the connection to be able to reuse
// it: the deadline must stay to bound that.
func TestRequestBody_Read_KeepsTheDeadlineOfAFailedRead(t *testing.T) {
	recorder := newConnRecorder()
	body := newTrackedBody(t, &scriptedBody{chunks: []string{"one"}, err: errClientGone}, recorder)
	body.renewReadDeadline(time.Minute)

	_, err := io.ReadAll(body)

	require.ErrorIs(t, err, errClientGone)
	require.NotErrorIs(t, err, context.DeadlineExceeded)
	deadlines := recorder.deadlines()
	assert.Len(t, deadlines, 3, "expected one deadline for the renewal and one per read")
	for _, deadline := range deadlines {
		assert.False(t, deadline.IsZero(), "expected the deadline to be kept")
	}
	_, _ = body.Read(make([]byte, 1))
	assert.Len(t, recorder.deadlines(), 3, "expected no deadline after the failed read")
}

func TestRequestBody_RenewReadDeadline_DoesNothing(t *testing.T) {
	tests := []struct {
		prepare           func(t *testing.T, body *requestBody)
		deadlineErr       error
		name              string
		timeout           time.Duration
		expectedDeadlines int
	}{
		{
			name:    "body read to the end should keep the connection as it is",
			timeout: time.Minute,
			prepare: func(t *testing.T, body *requestBody) {
				t.Helper()
				_, err := io.ReadAll(body)
				require.NoError(t, err)
			},
		},
		{
			name:    "closed body should keep the connection as it is",
			timeout: time.Minute,
			prepare: func(t *testing.T, body *requestBody) {
				t.Helper()
				require.NoError(t, body.Close())
			},
		},
		{
			name:    "released body should keep the connection as it is",
			timeout: time.Minute,
			prepare: func(_ *testing.T, body *requestBody) { body.release() },
		},
		{
			name:    "no timeout should keep the server read timeout",
			timeout: 0,
			prepare: func(*testing.T, *requestBody) {},
		},
		{
			// The attempt is the only call: nothing is renewed after it.
			name:              "response writer without read deadlines should keep the server read timeout",
			timeout:           time.Minute,
			deadlineErr:       http.ErrNotSupported,
			prepare:           func(*testing.T, *requestBody) {},
			expectedDeadlines: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := newConnRecorder()
			recorder.deadlineErr = tt.deadlineErr
			body := newTrackedBody(t, &scriptedBody{chunks: []string{"one", "two"}}, recorder)
			tt.prepare(t, body)

			body.renewReadDeadline(tt.timeout)
			_, _ = io.ReadAll(body)

			assert.Len(t, recorder.deadlines(), tt.expectedDeadlines)
			assert.Zero(t, recorder.fullDuplexCalls())
		})
	}
}

// A response writer that knows nothing about deadlines, like the ones of a middleware
// that does not unwrap, must leave the body as it was.
func TestRequestBody_WithoutDeadlineSupport(t *testing.T) {
	body := newTrackedBody(t, &scriptedBody{chunks: []string{"one", "two"}}, httptest.NewRecorder())

	body.renewReadDeadline(time.Minute)
	body.Abandon()
	content, err := io.ReadAll(body)

	require.NoError(t, err)
	assert.Equal(t, "onetwo", string(content))
}

func TestRequestBody_Abandon(t *testing.T) {
	t.Run("unfinished body should fail its reads at once with the timeout", func(t *testing.T) {
		recorder := newConnRecorder()
		body := newTrackedBody(t, &scriptedBody{err: errClientGone}, recorder)

		body.Abandon()
		_, err := body.Read(make([]byte, 1))

		deadlines := recorder.deadlines()
		require.Len(t, deadlines, 1)
		assert.False(t, deadlines[0].IsZero())
		assert.True(t, deadlines[0].Before(time.Now()), "expected a deadline in the past")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorIs(t, err, errClientGone)
	})
	t.Run("end of an abandoned body should stay the end of the body", func(t *testing.T) {
		body := newTrackedBody(t, &scriptedBody{}, newConnRecorder())

		body.Abandon()
		_, err := body.Read(make([]byte, 1))

		assert.Equal(t, io.EOF, err)
	})
	t.Run("body read to the end should keep the connection as it is", func(t *testing.T) {
		recorder := newConnRecorder()
		body := newTrackedBody(t, &scriptedBody{chunks: []string{"one"}}, recorder)
		_, err := io.ReadAll(body)
		require.NoError(t, err)

		body.Abandon()

		assert.Empty(t, recorder.deadlines())
	})
	t.Run("closed body should keep the connection as it is", func(t *testing.T) {
		recorder := newConnRecorder()
		body := newTrackedBody(t, &scriptedBody{chunks: []string{"one"}}, recorder)
		require.NoError(t, body.Close())

		body.Abandon()

		assert.Empty(t, recorder.deadlines())
	})
	t.Run("read that fails on its own should keep its error", func(t *testing.T) {
		body := newTrackedBody(t, &scriptedBody{err: errClientGone}, newConnRecorder())

		_, err := body.Read(make([]byte, 1))

		assert.Equal(t, errClientGone, err)
	})
}

// Once the handler is done the connection belongs to net/http again: a body that is
// still being read by the http transport must not set a deadline on it.
func TestRequestBody_Release(t *testing.T) {
	recorder := newConnRecorder()
	body := newTrackedBody(t, &scriptedBody{chunks: []string{"one", "two"}}, recorder)
	body.renewReadDeadline(time.Minute)

	body.release()
	content, err := io.ReadAll(body)
	body.Abandon()

	require.NoError(t, err)
	assert.Equal(t, "onetwo", string(content))
	deadlines := recorder.deadlines()
	require.Len(t, deadlines, 1, "expected the deadline of the renewal alone")
	assert.False(t, deadlines[0].IsZero(), "expected the last deadline to be kept")
}

func TestRequestBody_Close(t *testing.T) {
	recorder := newConnRecorder()
	source := &scriptedBody{chunks: []string{"one", "two"}}
	body := newTrackedBody(t, source, recorder)
	body.renewReadDeadline(time.Minute)

	require.NoError(t, body.Close())
	_, _ = io.ReadAll(body)

	assert.Equal(t, 1, source.closes)
	deadlines := recorder.deadlines()
	require.Len(t, deadlines, 1, "expected the deadline of the renewal alone")
	assert.False(t, deadlines[0].IsZero(), "expected the last deadline to be kept")
}

func TestRequestBody_NoBody(t *testing.T) {
	var body *requestBody

	assert.NotPanics(t, func() {
		body.renewReadDeadline(time.Minute)
		body.Abandon()
		body.release()
	})
}

// The http transport reads the body on its own goroutine while the handler renews the
// deadline and returns, and the gateway abandons the body from yet another one.
func TestRequestBody_ConcurrentUse(t *testing.T) {
	recorder := newConnRecorder()
	chunks := make([]string, 1000)
	for i := range chunks {
		chunks[i] = "chunk"
	}
	body := newTrackedBody(t, &scriptedBody{chunks: chunks}, recorder)
	read := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, body)
		read <- err
	}()
	abandoned := make(chan struct{})
	go func() {
		defer close(abandoned)
		body.Abandon()
	}()

	body.renewReadDeadline(time.Minute)
	body.release()

	require.NoError(t, <-read)
	<-abandoned
	last := len(recorder.deadlines())
	_, _ = body.Read(make([]byte, 1))
	assert.Len(t, recorder.deadlines(), last, "expected no deadline once released")
}
