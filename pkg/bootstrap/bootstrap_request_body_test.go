package bootstrap_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
)

// bodyTestWait is how long these tests wait for something that must happen. It is far
// shorter than the server read timeout of the gateways under test, 10 seconds unless a
// test lowers it, which is what a request body that is not read to the end used to wait
// for.
const bodyTestWait = 2 * time.Second

// bodyTestProtocols are the protocols a client can talk to the gateway.
var bodyTestProtocols = []struct {
	name  string
	http2 bool
}{
	{name: "HTTP/1.1", http2: false},
	{name: "HTTP/2", http2: true},
}

// bodyTestGateway describes a gateway with a single route to a backend.
type bodyTestGateway struct {
	backendURL string
	// routeExtra is more JSON for the route, such as its filters, starting with a comma.
	routeExtra string
	timeout    time.Duration
	// readTimeout replaces the read timeout of the initialized server when it is set.
	// Initialize never leaves it under the route timeout plus five seconds: a test
	// that needs a request body to outlive it lowers it instead of lasting that long.
	readTimeout time.Duration
	streaming   bool
	http2       bool
}

// start serves the gateway built by bootstrap.Initialize on a local listener and returns
// its URL and a client for it. HTTP/2 is served over TLS.
func (g bodyTestGateway) start(t *testing.T) (string, *http.Client) {
	t.Helper()
	cfgJSON := fmt.Sprintf(`{"gateway":{"routes":[{"id":"r1","uri":%q,"timeout":%q,"streaming":%t%s}]}}`,
		g.backendURL, g.timeout.String(), g.streaming, g.routeExtra)
	cfg, err := config.NewReaderJSON(validator.New()).Read([]byte(cfgJSON))
	require.NoError(t, err)
	server, err := bootstrap.Initialize(
		bootstrap.NewOptionsBuilder(cfg).WithLogger(slog.New(slog.DiscardHandler)).Build())
	require.NoError(t, err)
	if g.readTimeout > 0 {
		server.ReadTimeout = g.readTimeout
	}
	if g.http2 {
		testServer := httptest.NewUnstartedServer(server.Handler)
		testServer.Config = server
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)
		return testServer.URL, testServer.Client()
	}
	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	return serveBodyTest(t, server), &http.Client{Transport: transport}
}

// serveBodyTest serves the given server on a local listener and returns its URL.
//
// The server is closed when the test ends, without the wait of a graceful shutdown or
// of an httptest server: net/http keeps a connection whose request body it gave up on
// open for half a second, which these tests would otherwise spend on every exchange.
func serveBodyTest(t *testing.T, server *http.Server) string {
	t.Helper()
	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	return "http://" + listener.Addr().String()
}

// newBodyTestBackend serves the given handler and returns its URL.
func newBodyTestBackend(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	return serveBodyTest(t, &http.Server{Handler: handler, ReadHeaderTimeout: bodyTestWait})
}

// answerAfterTheBody answers once it has read the whole request body, like most upload
// endpoints do.
func answerAfterTheBody(writer http.ResponseWriter, request *http.Request) {
	if _, err := io.Copy(io.Discard, request.Body); err != nil {
		return
	}
	writer.WriteHeader(http.StatusOK)
}

// startAnswer sends the response headers at once, while the request body is still
// arriving. It reports false when the response writer cannot do that.
func startAnswer(writer http.ResponseWriter) bool {
	controller := http.NewResponseController(writer)
	if err := controller.EnableFullDuplex(); err != nil {
		return false
	}
	writer.WriteHeader(http.StatusOK)
	return controller.Flush() == nil
}

// echoTheBody answers at once and sends back every piece of the request body as it
// arrives.
func echoTheBody(writer http.ResponseWriter, request *http.Request) {
	if !startAnswer(writer) {
		return
	}
	controller := http.NewResponseController(writer)
	buf := make([]byte, 256)
	for {
		read, err := request.Body.Read(buf)
		if read > 0 {
			if _, writeErr := writer.Write(buf[:read]); writeErr != nil {
				return
			}
			if flushErr := controller.Flush(); flushErr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// beat writes a line to the response every interval until the given number of lines has
// been written, the request is cancelled or the client is gone. A negative count means
// no limit.
func beat(writer http.ResponseWriter, request *http.Request, interval time.Duration, count int) {
	controller := http.NewResponseController(writer)
	for sent := 0; count < 0 || sent < count; sent++ {
		select {
		case <-request.Context().Done():
			return
		case <-time.After(interval):
		}
		if _, err := io.WriteString(writer, "beat\n"); err != nil {
			return
		}
		if err := controller.Flush(); err != nil {
			return
		}
	}
}

// slowUpload is a request sent over a bare connection whose body is written one byte
// per interval.
type slowUpload struct {
	// size is the length of the body.
	size int
	// stallAfter is the number of bytes after which the client stops sending without
	// closing the connection. Zero means it sends the whole body.
	stallAfter int
	interval   time.Duration
	chunked    bool
	// keptOpen says that the server is expected to keep the connection: the client
	// does not wait for it to be closed after the response.
	keptOpen bool
}

// slowUploadReply is what the client of a slowUpload got back.
type slowUploadReply struct {
	// connErr is the error that ended the connection after the response.
	connErr error
	status  int
	// sent is the number of body bytes the client had written when the response arrived.
	sent int
	// closes is whether the response announced that the connection is closed.
	closes bool
}

// send writes the request to the given address and reads the response while the body is
// still being written, which is what tells when the response is sent.
func (u slowUpload) send(t *testing.T, addr string) slowUploadReply {
	t.Helper()
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(t.Context(), "tcp", addr)
	require.NoError(t, err)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(bodyTestWait)))
	head := "POST /upload HTTP/1.1\r\nHost: gateway\r\n"
	piece := "x"
	if u.chunked {
		head += "Transfer-Encoding: chunked\r\n\r\n"
		piece = "1\r\nx\r\n"
	} else {
		head += fmt.Sprintf("Content-Length: %d\r\n\r\n", u.size)
	}
	_, err = io.WriteString(conn, head)
	require.NoError(t, err)

	var sent atomic.Int64
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		toSend := u.size
		if u.stallAfter > 0 {
			toSend = u.stallAfter
		}
		for range toSend {
			time.Sleep(u.interval)
			if _, writeErr := io.WriteString(conn, piece); writeErr != nil {
				return
			}
			sent.Add(1)
		}
		if u.chunked && u.stallAfter == 0 {
			_, _ = io.WriteString(conn, "0\r\n\r\n")
		}
	}()
	defer func() {
		_ = conn.Close()
		<-writerDone
	}()

	res, err := readRawResponse(conn)
	require.NoError(t, err, "no response while the request body was being sent")
	reply := slowUploadReply{status: res.status, closes: res.closes(), sent: int(sent.Load())}
	if u.keptOpen {
		return reply
	}
	_, reply.connErr = conn.Read(make([]byte, 1))
	return reply
}

// rawResponse is a response read off a bare connection.
type rawResponse struct {
	header http.Header
	status int
}

// closes reports whether the response announces that the connection is closed.
func (r rawResponse) closes() bool {
	return strings.EqualFold(r.header.Get("Connection"), "close")
}

// readRawResponse reads a response off a bare connection: its head and the body its
// Content-Length announces, which is how the gateway sends its own responses.
func readRawResponse(conn net.Conn) (rawResponse, error) {
	var head []byte
	next := make([]byte, 1)
	for !bytes.HasSuffix(head, []byte("\r\n\r\n")) {
		if _, err := io.ReadFull(conn, next); err != nil {
			return rawResponse{}, fmt.Errorf("reading the response head after %q: %w", head, err)
		}
		head = append(head, next[0])
	}
	lines := strings.Split(strings.TrimSuffix(string(head), "\r\n\r\n"), "\r\n")
	statusLine := strings.Fields(lines[0])
	if len(statusLine) < 2 {
		return rawResponse{}, fmt.Errorf("malformed status line %q", lines[0])
	}
	status, err := strconv.Atoi(statusLine[1])
	if err != nil {
		return rawResponse{}, fmt.Errorf("malformed status line %q: %w", lines[0], err)
	}
	res := rawResponse{status: status, header: http.Header{}}
	for _, line := range lines[1:] {
		name, value, _ := strings.Cut(line, ":")
		res.header.Add(name, strings.TrimSpace(value))
	}
	length, err := strconv.Atoi(res.header.Get("Content-Length"))
	if err != nil {
		return rawResponse{}, fmt.Errorf("response without a Content-Length: %w", err)
	}
	if _, err = io.ReadFull(conn, make([]byte, length)); err != nil {
		return rawResponse{}, fmt.Errorf("reading the response body: %w", err)
	}
	return res, nil
}

// requireNoRequestBodyReader fails when a goroutine keeps reading a request body through
// the gateway, which is what the goroutine of the http transport that forwards a body to
// the backend does until the client sends something or the server read timeout expires.
func requireNoRequestBodyReader(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		stacks := make([]byte, 4<<20)
		stacks = stacks[:runtime.Stack(stacks, true)]
		return !strings.Contains(string(stacks), "gateway.(*ReplayableBody).Read")
	}, bodyTestWait, 10*time.Millisecond, "a goroutine is still reading the request body")
}

// net/http reads what is left of a request body before it writes the response, and the
// http transport does not return before the read of the body it is waiting on does: the
// 504 of a route that timed out during an upload was only sent when the upload ended or
// the server read timeout expired.
func TestInitialize_RouteTimeoutIsAnsweredWhileTheRequestBodyIsSent(t *testing.T) {
	const routeTimeout = 100 * time.Millisecond
	tests := []struct {
		name      string
		upload    slowUpload
		streaming bool
	}{
		{
			name:   "upload of known length slower than the route timeout",
			upload: slowUpload{size: 40, interval: 20 * time.Millisecond},
		},
		{
			name:   "chunked upload slower than the route timeout",
			upload: slowUpload{size: 40, interval: 20 * time.Millisecond, chunked: true},
		},
		{
			name:   "upload of a client that stops sending",
			upload: slowUpload{size: 40, interval: 5 * time.Millisecond, stallAfter: 3},
		},
		{
			name:   "large upload of a client that stops sending",
			upload: slowUpload{size: 1 << 20, interval: 5 * time.Millisecond, stallAfter: 3},
		},
		{
			// The route timeout of a streaming route runs until the response is
			// ready: an upload the backend only answers at its end has to fit in it.
			name:      "upload slower than the route timeout on a streaming route",
			upload:    slowUpload{size: 40, interval: 20 * time.Millisecond, chunked: true},
			streaming: true,
		},
		{
			name:      "upload of a client that stops sending on a streaming route",
			upload:    slowUpload{size: 40, interval: 5 * time.Millisecond, stallAfter: 3},
			streaming: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newBodyTestBackend(t, answerAfterTheBody)
			gatewayURL, _ := bodyTestGateway{
				backendURL: backend,
				timeout:    routeTimeout,
				streaming:  tt.streaming,
			}.start(t)
			start := time.Now()

			reply := tt.upload.send(t, strings.TrimPrefix(gatewayURL, "http://"))

			assert.Equal(t, http.StatusGatewayTimeout, reply.status)
			assert.GreaterOrEqual(t, time.Since(start), routeTimeout)
			assert.Less(t, reply.sent, tt.upload.size, "the response was only sent once the upload ended")
			// The rest of the body is not read, so the connection cannot be reused:
			// the server says so and closes it, instead of waiting for the client.
			assert.True(t, reply.closes, "expected Connection: close")
			require.Error(t, reply.connErr)
			if netErr, ok := errors.AsType[net.Error](reply.connErr); ok {
				assert.False(t, netErr.Timeout(), "the server kept the connection open: %v", reply.connErr)
			}
			requireNoRequestBodyReader(t)
		})
	}
}

// pacedBody is a request body that hands out one byte per interval and then, when it
// stalls, blocks until it is closed or the request is cancelled: the http transport of
// the client does not return before the read of the body it is waiting on does.
type pacedBody struct {
	cancelled <-chan struct{}
	closed    chan struct{}
	sent      atomic.Int64
	size      int64
	interval  time.Duration
	once      sync.Once
	stalls    bool
}

func newPacedBody(ctx context.Context, size int64, interval time.Duration, stalls bool) *pacedBody {
	return &pacedBody{
		cancelled: ctx.Done(),
		closed:    make(chan struct{}),
		size:      size,
		interval:  interval,
		stalls:    stalls,
	}
}

func (b *pacedBody) Read(output []byte) (int, error) {
	wait := time.After(b.interval)
	if b.sent.Load() == b.size {
		if !b.stalls {
			return 0, io.EOF
		}
		wait = nil
	}
	select {
	case <-b.cancelled:
		return 0, context.Canceled
	case <-b.closed:
		return 0, io.ErrClosedPipe
	case <-wait:
	}
	output[0] = 'x'
	b.sent.Add(1)
	return 1, nil
}

func (b *pacedBody) Close() error {
	b.once.Do(func() { close(b.closed) })
	return nil
}

func (b *pacedBody) isClosed() bool {
	select {
	case <-b.closed:
		return true
	default:
		return false
	}
}

// A Go client reads the response while it is still writing the request body, and stops
// writing it then.
func TestInitialize_RouteTimeoutIsAnsweredToAGoClientThatIsStillSending(t *testing.T) {
	const routeTimeout = 100 * time.Millisecond
	tests := []struct {
		name        string
		size        int64
		knownLength bool
		stalls      bool
		http2       bool
	}{
		{name: "HTTP/1.1 upload of known length slower than the route timeout", size: 40, knownLength: true},
		{name: "HTTP/1.1 upload of unknown length slower than the route timeout", size: 40},
		{name: "HTTP/1.1 upload of a client that stops sending", size: 3, stalls: true},
		{name: "HTTP/2 upload of a client that stops sending", size: 3, stalls: true, http2: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newBodyTestBackend(t, answerAfterTheBody)
			gatewayURL, client := bodyTestGateway{
				backendURL: backend,
				timeout:    routeTimeout,
				http2:      tt.http2,
			}.start(t)
			ctx, cancel := context.WithTimeout(t.Context(), bodyTestWait)
			defer cancel()
			body := newPacedBody(ctx, tt.size, 20*time.Millisecond, tt.stalls)
			request, err := http.NewRequestWithContext(ctx, http.MethodPost, gatewayURL+"/upload", body)
			require.NoError(t, err)
			if tt.knownLength {
				request.ContentLength = tt.size
			}

			res, err := client.Do(request)

			require.NoError(t, err, "no response while the request body was being sent")
			sent := body.sent.Load()
			_, err = io.Copy(io.Discard, res.Body)
			require.NoError(t, err)
			require.NoError(t, res.Body.Close())
			assert.Equal(t, http.StatusGatewayTimeout, res.StatusCode)
			if !tt.stalls {
				assert.Less(t, sent, tt.size, "the response was only sent once the upload ended")
				// The client gives up on the rest of the body once it has the response.
				// One that is blocked in a read of the body only does when that returns.
				assert.Eventually(t, body.isClosed, bodyTestWait, 5*time.Millisecond,
					"the client kept sending the request body")
			}
			requireNoRequestBodyReader(t)
		})
	}
}

// A request body that was read to the end is not abandoned: the connection is kept.
func TestInitialize_RouteTimeoutKeepsTheConnectionOfARequestBodyReadToTheEnd(t *testing.T) {
	// The backend reads the body and never answers.
	backend := newBodyTestBackend(t, func(_ http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		<-request.Context().Done()
	})
	gatewayURL, _ := bodyTestGateway{backendURL: backend, timeout: 100 * time.Millisecond}.start(t)
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(t.Context(), "tcp", strings.TrimPrefix(gatewayURL, "http://"))
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck
	require.NoError(t, conn.SetDeadline(time.Now().Add(bodyTestWait)))

	// The second request is only answered if the connection outlived the first one.
	for range 2 {
		_, err = io.WriteString(conn, "POST /upload HTTP/1.1\r\nHost: gateway\r\nContent-Length: 5\r\n\r\nhello")
		require.NoError(t, err)
		res, err := readRawResponse(conn)
		require.NoError(t, err)

		assert.Equal(t, http.StatusGatewayTimeout, res.status)
		assert.False(t, res.closes(), "expected the connection to be kept")
	}
}

// upload is the request body of a client that talks to a streaming route: lines written
// one per interval.
type upload struct {
	reader *io.PipeReader
	// done is closed once the last line has been written.
	done chan struct{}
	// length is the length the request declares. Zero means it does not declare one.
	length int64
}

// uploadLines is what an upload of the given number of lines sends.
func uploadLines(lines int) string {
	var content strings.Builder
	for line := range lines {
		fmt.Fprintf(&content, "line %d\n", line)
	}
	return content.String()
}

// withLength makes the request of the upload declare the length of the given number of
// lines, which is how net/http and the http transport know where the body ends without
// reading past it.
func (u *upload) withLength(lines int) *upload {
	u.length = int64(len(uploadLines(lines)))
	return u
}

// startUpload writes the given number of lines, one per interval, and then ends the body
// unless keepOpen is set, in which case the client just stops sending. The body fails
// once the given context is done: the http transport of the client does not return
// before the read of the body it is waiting on does.
func startUpload(ctx context.Context, lines int, interval time.Duration, keepOpen bool) *upload {
	reader, writer := io.Pipe()
	body := &upload{reader: reader, done: make(chan struct{})}
	context.AfterFunc(ctx, func() { _ = writer.CloseWithError(context.Cause(ctx)) })
	go func() {
		defer close(body.done)
		for line := range lines {
			time.Sleep(interval)
			if _, err := fmt.Fprintf(writer, "line %d\n", line); err != nil {
				return
			}
		}
		if !keepOpen {
			_ = writer.Close()
		}
	}()
	return body
}

// post sends the upload and returns the response once its headers are in. The caller
// closes its body.
func (u *upload) post(ctx context.Context, t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, u.reader)
	require.NoError(t, err)
	request.ContentLength = u.length
	res, err := client.Do(request)
	require.NoError(t, err)
	if !assert.Equal(t, http.StatusOK, res.StatusCode) {
		_ = res.Body.Close()
		t.FailNow()
	}
	return res
}

// The server read timeout is a deadline for the whole request. Once the response of a
// streaming route has started it is replaced by a deadline per read of the request body,
// so a client that keeps sending is not cut. On HTTP/1.1 the body did not even get that
// far: net/http read what was left of it and threw it away to write the response.
func TestInitialize_StreamingRouteRequestBodyOutlivesTheServerReadTimeout(t *testing.T) {
	const (
		readTimeout = 150 * time.Millisecond
		lines       = 12
		interval    = 25 * time.Millisecond
	)
	for _, protocol := range bodyTestProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			backend := newBodyTestBackend(t, echoTheBody)
			gatewayURL, client := bodyTestGateway{
				backendURL:  backend,
				timeout:     400 * time.Millisecond,
				readTimeout: readTimeout,
				streaming:   true,
				http2:       protocol.http2,
			}.start(t)
			ctx, cancel := context.WithTimeout(t.Context(), bodyTestWait)
			defer cancel()
			start := time.Now()

			res := startUpload(ctx, lines, interval, false).post(ctx, t, client, gatewayURL+"/echo")
			defer res.Body.Close() //nolint:errcheck
			echoed, err := io.ReadAll(res.Body)

			require.NoError(t, err)
			assert.Equal(t, uploadLines(lines), string(echoed))
			assert.Greater(t, time.Since(start), readTimeout)
		})
	}
}

// The deadline of every read is the route timeout: a client that stops sending for
// longer than that is dropped, long before the server read timeout would do it.
func TestInitialize_StreamingRouteRequestBodyOfAStalledClientIsCut(t *testing.T) {
	const routeTimeout = 150 * time.Millisecond
	for _, protocol := range bodyTestProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			// The backend keeps the response busy, so that the write deadline of the
			// streaming route is not the one that cuts the exchange.
			backend := newBodyTestBackend(t, func(writer http.ResponseWriter, request *http.Request) {
				if !startAnswer(writer) {
					return
				}
				go func() { _, _ = io.Copy(io.Discard, request.Body) }()
				beat(writer, request, 20*time.Millisecond, -1)
			})
			gatewayURL, client := bodyTestGateway{
				backendURL: backend,
				timeout:    routeTimeout,
				streaming:  true,
				http2:      protocol.http2,
			}.start(t)
			ctx, cancel := context.WithTimeout(t.Context(), bodyTestWait)
			defer cancel()

			body := startUpload(ctx, 3, 10*time.Millisecond, true)
			res := body.post(ctx, t, client, gatewayURL+"/stall")
			defer res.Body.Close() //nolint:errcheck
			<-body.done
			stalled := time.Now()
			_, err := io.Copy(io.Discard, res.Body)

			require.Error(t, err, "expected the exchange to be cut")
			require.NoError(t, ctx.Err(), "the exchange outlived a client that stopped sending")
			assert.GreaterOrEqual(t, time.Since(stalled), routeTimeout*2/3, "cut before the route timeout")
		})
	}
}

// At the end of a request body net/http stops the read deadline of the connection and
// starts detecting a client that goes away. A deadline renewed after that would cancel
// the request once it expired, or the next request of the connection.
func TestInitialize_StreamingRouteRequestBodyReadToTheEndLeavesNoDeadline(t *testing.T) {
	const (
		routeTimeout = 100 * time.Millisecond
		beats        = 6
		interval     = 30 * time.Millisecond
	)
	for _, protocol := range bodyTestProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			backend := newBodyTestBackend(t, func(writer http.ResponseWriter, request *http.Request) {
				if !startAnswer(writer) {
					return
				}
				if _, err := io.Copy(io.Discard, request.Body); err != nil {
					return
				}
				beat(writer, request, interval, beats)
			})
			gatewayURL, client := bodyTestGateway{
				backendURL: backend,
				timeout:    routeTimeout,
				streaming:  true,
				http2:      protocol.http2,
			}.start(t)
			reused := false
			ctx, cancel := context.WithTimeout(
				httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
					GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused },
				}), bodyTestWait)
			defer cancel()

			// The second exchange runs on the connection of the first one.
			for exchange := range 2 {
				start := time.Now()
				// The http transport reads once more after the end of a body of
				// known length, to make sure it is the end.
				body := startUpload(ctx, 3, 10*time.Millisecond, false).withLength(3)
				res := body.post(ctx, t, client, gatewayURL+"/beats")
				received, err := io.ReadAll(res.Body)
				require.NoError(t, res.Body.Close())

				require.NoError(t, err, "exchange %d", exchange)
				assert.Equal(t, strings.Repeat("beat\n", beats), string(received), "exchange %d", exchange)
				// A deadline left behind would have expired one route timeout after
				// the end of the request body.
				assert.Greater(t, time.Since(start), 3*routeTimeout/2, "exchange %d", exchange)
			}
			assert.True(t, reused, "expected the second exchange to reuse the connection")
		})
	}
}

// The request body being read to the end must leave the detection of a client that goes
// away working: the backend request is cancelled when it does.
func TestInitialize_StreamingRouteDetectsAClientGoneAfterItsRequestBody(t *testing.T) {
	const routeTimeout = 100 * time.Millisecond
	for _, protocol := range bodyTestProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			gone := make(chan struct{})
			backend := newBodyTestBackend(t, func(writer http.ResponseWriter, request *http.Request) {
				if !startAnswer(writer) {
					return
				}
				if _, err := io.Copy(io.Discard, request.Body); err != nil {
					return
				}
				beat(writer, request, 20*time.Millisecond, -1)
				close(gone)
			})
			gatewayURL, client := bodyTestGateway{
				backendURL: backend,
				timeout:    routeTimeout,
				streaming:  true,
				http2:      protocol.http2,
			}.start(t)
			ctx, cancel := context.WithTimeout(t.Context(), bodyTestWait)
			defer cancel()

			res := startUpload(ctx, 3, 10*time.Millisecond, false).post(ctx, t, client, gatewayURL+"/beats")
			defer res.Body.Close() //nolint:errcheck
			// Twelve beats end well past the route timeout after the end of the request body.
			_, err := io.ReadFull(res.Body, make([]byte, 12*len("beat\n")))
			require.NoError(t, err, "the response was cut")
			select {
			case <-gone:
				require.Fail(t, "the backend request ended while the client was reading")
			default:
			}
			cancel()

			select {
			case <-gone:
			case <-time.After(bodyTestWait):
				require.Fail(t, "the backend request outlived the client")
			}
		})
	}
}
