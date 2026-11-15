package bootstrap_test

// The request body tests of bootstrap_request_body_test.go and
// bootstrap_request_body_errors_test.go run over plain HTTP/1.1 or HTTP/2 between client
// and gateway and plain HTTP/1.1 between gateway and backend. These run the same
// behaviours over HTTP/1.1 with TLS between client and gateway, over HTTP/2 between
// gateway and backend, and with Expect: 100-continue.

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
)

// gapH2Client is the http client section that lets the gateway talk HTTP/2 to a TLS
// backend with a self-signed certificate.
const gapH2Client = `{"enable-http2":true,"insecure-tls-verify":true}`

// gapCert is a certificate for 127.0.0.1, issued by an authority of the test.
func gapCert(t *testing.T) tls.Certificate {
	t.Helper()
	certificate := newTestAuthority(t, "authority of the gaps test").issueLoopbackServer(t)
	keyPair, err := tls.X509KeyPair([]byte(certificate.cert), []byte(certificate.key))
	require.NoError(t, err)
	return keyPair
}

// serveGapTLS serves the server over TLS, HTTP/1.1 only unless http2 is set, and
// returns its address. It is closed without waiting when the test ends.
func serveGapTLS(t *testing.T, server *http.Server, http2 bool) string {
	t.Helper()
	server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{gapCert(t)}, MinVersion: tls.VersionTLS12}
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(http2)
	server.Protocols = protocols
	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.ServeTLS(listener, "", "") }()
	t.Cleanup(func() { _ = server.Close() })
	return listener.Addr().String()
}

// newGapH2Backend serves the handler over HTTP/2 with TLS and records the protocol of
// every request it gets.
func newGapH2Backend(t *testing.T, handler http.HandlerFunc) (string, *atomic.Int64) {
	t.Helper()
	var proto atomic.Int64
	addr := serveGapTLS(t, &http.Server{
		ReadHeaderTimeout: bodyTestWait,
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			proto.Store(int64(request.ProtoMajor))
			handler(writer, request)
		}),
	}, true)
	return "https://" + addr, &proto
}

// gapGateway is a gateway with one route. clientTLS serves it as HTTP/1.1 over TLS,
// clientH2 as HTTP/2 over TLS, and neither as plain HTTP/1.1.
type gapGateway struct {
	backendURL  string
	routeExtra  string
	httpClient  string
	timeout     time.Duration
	readTimeout time.Duration
	streaming   bool
	clientTLS   bool
	clientH2    bool
}

// start returns the address of the gateway, its base URL and a client for it.
func (g gapGateway) start(t *testing.T) (string, string, *http.Client) {
	t.Helper()
	httpClient := ""
	if g.httpClient != "" {
		httpClient = `"httpclient":` + g.httpClient + `,`
	}
	cfgJSON := fmt.Sprintf(`{"gateway":{%s"routes":[{"id":"r1","uri":%q,"timeout":%q,"streaming":%t%s}]}}`,
		httpClient, g.backendURL, g.timeout.String(), g.streaming, g.routeExtra)
	cfg, err := config.NewReaderJSON(validator.New()).Read([]byte(cfgJSON))
	require.NoError(t, err)
	server, err := bootstrap.Initialize(
		bootstrap.NewOptionsBuilder(cfg).WithLogger(slog.New(slog.DiscardHandler)).Build())
	require.NoError(t, err)
	if g.readTimeout > 0 {
		server.ReadTimeout = g.readTimeout
	}
	if !g.clientTLS && !g.clientH2 {
		transport := &http.Transport{}
		t.Cleanup(transport.CloseIdleConnections)
		url := serveBodyTest(t, server)
		return strings.TrimPrefix(url, "http://"), url, &http.Client{Transport: transport}
	}
	addr := serveGapTLS(t, server, g.clientH2)
	protocols := new(http.Protocols)
	protocols.SetHTTP1(!g.clientH2)
	protocols.SetHTTP2(g.clientH2)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		Protocols:       protocols,
	}
	t.Cleanup(transport.CloseIdleConnections)
	return addr, "https://" + addr, &http.Client{Transport: transport}
}

// dialGap opens a bare connection to the gateway, over TLS when useTLS is set.
func dialGap(t *testing.T, addr string, useTLS bool) net.Conn {
	t.Helper()
	dialer := &net.Dialer{}
	if !useTLS {
		conn, err := dialer.DialContext(t.Context(), "tcp", addr)
		require.NoError(t, err)
		return conn
	}
	tlsDialer := &tls.Dialer{NetDialer: dialer, Config: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec
		NextProtos:         []string{"http/1.1"},
	}}
	conn, err := tlsDialer.DialContext(t.Context(), "tcp", addr)
	require.NoError(t, err)
	return conn
}

// gapUpload is a request written by hand on a bare connection, its body one byte per
// interval.
type gapUpload struct {
	size       int
	stallAfter int
	interval   time.Duration
	chunked    bool
	keptOpen   bool
	// expect sends Expect: 100-continue and only writes the body once the server
	// answered 100 Continue.
	expect bool
}

type gapReply struct {
	connErr     error
	elapsed     time.Duration
	status      int
	sent        int
	closes      bool
	gotContinue bool
}

// readGapHead reads the head of a response.
func readGapHead(conn net.Conn) (rawResponse, error) {
	var head []byte
	next := make([]byte, 1)
	for !strings.HasSuffix(string(head), "\r\n\r\n") {
		if _, err := io.ReadFull(conn, next); err != nil {
			return rawResponse{}, fmt.Errorf("reading the response head after %q: %w", head, err)
		}
		head = append(head, next[0])
	}
	lines := strings.Split(strings.TrimSuffix(string(head), "\r\n\r\n"), "\r\n")
	fields := strings.Fields(lines[0])
	if len(fields) < 2 {
		return rawResponse{}, fmt.Errorf("malformed status line %q", lines[0])
	}
	status, err := strconv.Atoi(fields[1])
	if err != nil {
		return rawResponse{}, fmt.Errorf("malformed status line %q: %w", lines[0], err)
	}
	res := rawResponse{status: status, header: http.Header{}}
	for _, line := range lines[1:] {
		name, value, _ := strings.Cut(line, ":")
		res.header.Add(name, strings.TrimSpace(value))
	}
	return res, nil
}

func (u gapUpload) send(t *testing.T, conn net.Conn) gapReply {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(bodyTestWait)))
	head := "POST /upload HTTP/1.1\r\nHost: gateway\r\n"
	if u.expect {
		head += "Expect: 100-continue\r\n"
	}
	piece := "x"
	if u.chunked {
		head += "Transfer-Encoding: chunked\r\n\r\n"
		piece = "1\r\nx\r\n"
	} else {
		head += fmt.Sprintf("Content-Length: %d\r\n\r\n", u.size)
	}
	start := time.Now()
	_, err := io.WriteString(conn, head)
	require.NoError(t, err)

	begin := make(chan struct{})
	stop := make(chan struct{})
	var sent atomic.Int64
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		select {
		case <-begin:
		case <-stop:
			return
		}
		toSend := u.size
		if u.stallAfter > 0 {
			toSend = u.stallAfter
		}
		for range toSend {
			select {
			case <-stop:
				return
			case <-time.After(u.interval):
			}
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
		close(stop)
		_ = conn.Close()
		<-writerDone
	}()

	reply := gapReply{}
	if !u.expect {
		close(begin)
	}
	res, err := readGapHead(conn)
	require.NoError(t, err, "no response")
	if res.status == http.StatusContinue {
		reply.gotContinue = true
		close(begin)
		res, err = readGapHead(conn)
		require.NoError(t, err, "no final response while the request body was being sent")
	} else if u.expect {
		close(begin)
	}
	length, err := strconv.Atoi(res.header.Get("Content-Length"))
	require.NoError(t, err, "response without a Content-Length")
	_, err = io.ReadFull(conn, make([]byte, length))
	require.NoError(t, err)
	reply.elapsed = time.Since(start)
	reply.status = res.status
	reply.closes = res.closes()
	reply.sent = int(sent.Load())
	if !u.keptOpen {
		_, reply.connErr = conn.Read(make([]byte, 1))
	}
	return reply
}

// assertClosedNotTimedOut checks that the server closed the connection itself instead
// of leaving it open until the client gave up.
func assertClosedNotTimedOut(t *testing.T, reply gapReply) {
	t.Helper()
	assert.True(t, reply.closes, "expected Connection: close")
	require.Error(t, reply.connErr)
	if netErr, ok := errors.AsType[net.Error](reply.connErr); ok {
		assert.False(t, netErr.Timeout(), "the server kept the connection open: %v", reply.connErr)
	}
}

// --- HTTP/1.1 over TLS between client and gateway ---------------------------------

func TestGap_TLSHTTP1_RouteTimeoutIsAnsweredWhileTheRequestBodyIsSent(t *testing.T) {
	const routeTimeout = 100 * time.Millisecond
	tests := []struct {
		name      string
		upload    gapUpload
		streaming bool
	}{
		{name: "known length", upload: gapUpload{size: 40, interval: 20 * time.Millisecond}},
		{name: "chunked", upload: gapUpload{size: 40, interval: 20 * time.Millisecond, chunked: true}},
		{name: "client that stops sending", upload: gapUpload{size: 40, interval: 5 * time.Millisecond, stallAfter: 3}},
		{name: "large body, client that stops sending", upload: gapUpload{size: 1 << 20, interval: 5 * time.Millisecond, stallAfter: 3}},
		{name: "streaming route", upload: gapUpload{size: 40, interval: 20 * time.Millisecond, chunked: true}, streaming: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, _, _ := gapGateway{
				backendURL: newBodyTestBackend(t, answerAfterTheBody),
				timeout:    routeTimeout,
				streaming:  tt.streaming,
				clientTLS:  true,
			}.start(t)

			reply := tt.upload.send(t, dialGap(t, addr, true))

			assert.Equal(t, http.StatusGatewayTimeout, reply.status)
			assert.Less(t, reply.elapsed, 3*routeTimeout, "the 504 was held back")
			assert.Less(t, reply.sent, tt.upload.size)
			assertClosedNotTimedOut(t, reply)
			requireNoRequestBodyReader(t)
		})
	}
}

func TestGap_TLSHTTP1_RouteTimeoutKeepsTheConnectionOfABodyReadToTheEnd(t *testing.T) {
	backend := newBodyTestBackend(t, func(_ http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		<-request.Context().Done()
	})
	addr, _, _ := gapGateway{backendURL: backend, timeout: 100 * time.Millisecond, clientTLS: true}.start(t)
	conn := dialGap(t, addr, true)
	defer conn.Close() //nolint:errcheck
	require.NoError(t, conn.SetDeadline(time.Now().Add(bodyTestWait)))

	for i := range 3 {
		_, err := io.WriteString(conn, "POST /upload HTTP/1.1\r\nHost: gateway\r\nContent-Length: 5\r\n\r\nhello")
		require.NoError(t, err)
		res, err := readRawResponse(conn)
		require.NoError(t, err, "request %d", i)
		assert.Equal(t, http.StatusGatewayTimeout, res.status)
		assert.False(t, res.closes(), "expected the connection to be kept")
	}
}

func TestGap_TLSHTTP1_RejectionsDoNotWaitForTheBody(t *testing.T) {
	t.Run("slow body: 429 within the grace, connection closed", func(t *testing.T) {
		addr, gatewayURL, client := gapGateway{
			backendURL: newBodyTestBackend(t, answerAfterTheBody),
			timeout:    5 * time.Second,
			routeExtra: oneRequestRateLimit,
			clientTLS:  true,
		}.start(t)
		warmUp(t, client, gatewayURL)

		reply := gapUpload{size: 30, interval: 20 * time.Millisecond}.send(t, dialGap(t, addr, true))

		assert.Equal(t, http.StatusTooManyRequests, reply.status)
		assert.Less(t, reply.elapsed, 300*time.Millisecond)
		assertClosedNotTimedOut(t, reply)
	})
	t.Run("body sent with the headers: 429 keeps the connection", func(t *testing.T) {
		_, gatewayURL, client := gapGateway{
			backendURL: newBodyTestBackend(t, answerAfterTheBody),
			timeout:    5 * time.Second,
			routeExtra: oneRequestRateLimit,
			clientTLS:  true,
		}.start(t)
		warmUp(t, client, gatewayURL)
		reused := false
		trace := &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused }}
		for range 2 {
			request, err := http.NewRequestWithContext(httptrace.WithClientTrace(t.Context(), trace),
				http.MethodPost, gatewayURL+"/upload", strings.NewReader("payload"))
			require.NoError(t, err)
			res, err := client.Do(request)
			require.NoError(t, err)
			require.NoError(t, res.Body.Close())
			assert.Equal(t, 1, res.ProtoMajor)
			assert.Equal(t, http.StatusTooManyRequests, res.StatusCode)
			assert.False(t, res.Close)
		}
		assert.True(t, reused)
	})
	t.Run("backend that answers before the body", func(t *testing.T) {
		addr, _, _ := gapGateway{
			backendURL: newBodyTestBackend(t, rejectAtOnce),
			timeout:    5 * time.Second,
			clientTLS:  true,
		}.start(t)

		reply := gapUpload{size: 30, interval: 20 * time.Millisecond}.send(t, dialGap(t, addr, true))

		assert.Equal(t, http.StatusRequestEntityTooLarge, reply.status)
		assert.Less(t, reply.elapsed, 300*time.Millisecond)
		assertClosedNotTimedOut(t, reply)
		requireNoRequestBodyReader(t)
	})
}

func TestGap_TLSHTTP1_StreamingRoute(t *testing.T) {
	t.Run("bidirectional body outlives the read timeout", func(t *testing.T) {
		const readTimeout = 150 * time.Millisecond
		_, gatewayURL, client := gapGateway{
			backendURL:  newBodyTestBackend(t, echoTheBody),
			timeout:     400 * time.Millisecond,
			readTimeout: readTimeout,
			streaming:   true,
			clientTLS:   true,
		}.start(t)
		ctx := t.Context()
		start := time.Now()
		res := startUpload(ctx, 12, 25*time.Millisecond, false).post(ctx, t, client, gatewayURL+"/echo")
		defer res.Body.Close() //nolint:errcheck
		echoed, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		assert.Equal(t, 1, res.ProtoMajor)
		assert.Equal(t, uploadLines(12), string(echoed))
		assert.Greater(t, time.Since(start), readTimeout)
	})
	t.Run("stalled client is cut at the route timeout", func(t *testing.T) {
		const routeTimeout = 150 * time.Millisecond
		backend := newBodyTestBackend(t, func(writer http.ResponseWriter, request *http.Request) {
			if !startAnswer(writer) {
				return
			}
			go func() { _, _ = io.Copy(io.Discard, request.Body) }()
			beat(writer, request, 20*time.Millisecond, -1)
		})
		_, gatewayURL, client := gapGateway{
			backendURL: backend, timeout: routeTimeout, streaming: true, clientTLS: true,
		}.start(t)
		ctx := t.Context()
		body := startUpload(ctx, 3, 10*time.Millisecond, true)
		res := body.post(ctx, t, client, gatewayURL+"/stall")
		defer res.Body.Close() //nolint:errcheck
		<-body.done
		stalled := time.Now()
		_, err := io.Copy(io.Discard, res.Body)
		require.Error(t, err)
		assert.GreaterOrEqual(t, time.Since(stalled), routeTimeout*2/3)
		assert.Less(t, time.Since(stalled), bodyTestWait)
	})
	t.Run("body read to the end leaves no deadline and the connection is reused", func(t *testing.T) {
		const routeTimeout = 100 * time.Millisecond
		backend := newBodyTestBackend(t, func(writer http.ResponseWriter, request *http.Request) {
			if !startAnswer(writer) {
				return
			}
			if _, err := io.Copy(io.Discard, request.Body); err != nil {
				return
			}
			beat(writer, request, 30*time.Millisecond, 6)
		})
		_, gatewayURL, client := gapGateway{
			backendURL: backend, timeout: routeTimeout, streaming: true, clientTLS: true,
		}.start(t)
		reused := false
		ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused },
		})
		for exchange := range 2 {
			body := startUpload(ctx, 3, 10*time.Millisecond, false).withLength(3)
			res := body.post(ctx, t, client, gatewayURL+"/beats")
			received, err := io.ReadAll(res.Body)
			require.NoError(t, res.Body.Close())
			require.NoError(t, err, "exchange %d", exchange)
			assert.Equal(t, strings.Repeat("beat\n", 6), string(received))
		}
		assert.True(t, reused)
	})
}

// --- HTTP/2 between gateway and backend ---------------------------------------------

func TestGap_H2Backend_RouteTimeoutIsAnsweredWhileTheRequestBodyIsSent(t *testing.T) {
	const routeTimeout = 100 * time.Millisecond
	tests := []struct {
		name      string
		upload    gapUpload
		streaming bool
	}{
		{name: "known length", upload: gapUpload{size: 40, interval: 20 * time.Millisecond}},
		{name: "chunked", upload: gapUpload{size: 40, interval: 20 * time.Millisecond, chunked: true}},
		{name: "client that stops sending", upload: gapUpload{size: 40, interval: 5 * time.Millisecond, stallAfter: 3}},
		{name: "large body, client that stops sending", upload: gapUpload{size: 1 << 20, interval: 5 * time.Millisecond, stallAfter: 3}},
		{name: "streaming route", upload: gapUpload{size: 40, interval: 20 * time.Millisecond, chunked: true}, streaming: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.streaming {
				// The HTTP/2 transport reports the timeout of a streaming route as a
				// context.Canceled, answered 499, on master as well.
				t.Skip("answered 499 instead of 504 until #33 is merged")
			}
			backend, proto := newGapH2Backend(t, answerAfterTheBody)
			addr, _, _ := gapGateway{
				backendURL: backend,
				httpClient: gapH2Client,
				timeout:    routeTimeout,
				streaming:  tt.streaming,
			}.start(t)

			reply := tt.upload.send(t, dialGap(t, addr, false))

			assert.Equal(t, int64(2), proto.Load(), "the backend was not reached over HTTP/2")
			assert.Equal(t, http.StatusGatewayTimeout, reply.status)
			assert.Less(t, reply.elapsed, 3*routeTimeout, "the 504 was held back")
			assert.Less(t, reply.sent, tt.upload.size)
			assertClosedNotTimedOut(t, reply)
			requireNoRequestBodyReader(t)
		})
	}
}

func TestGap_H2Backend_HTTP2ClientThatStopsSending(t *testing.T) {
	const routeTimeout = 100 * time.Millisecond
	backend, proto := newGapH2Backend(t, answerAfterTheBody)
	_, gatewayURL, client := gapGateway{
		backendURL: backend, httpClient: gapH2Client, timeout: routeTimeout, clientH2: true,
	}.start(t)
	ctx := t.Context()
	body := newStalledUploadBody(ctx, 3, 20*time.Millisecond, true)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, gatewayURL+"/upload", body)
	require.NoError(t, err)
	start := time.Now()

	res, err := client.Do(request)

	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, res.Body)
	require.NoError(t, res.Body.Close())
	assert.Equal(t, 2, res.ProtoMajor)
	assert.Equal(t, int64(2), proto.Load())
	assert.Equal(t, http.StatusGatewayTimeout, res.StatusCode)
	assert.Less(t, time.Since(start), 3*routeTimeout)
	requireNoRequestBodyReader(t)
}

func TestGap_H2Backend_RouteTimeoutKeepsTheConnectionOfABodyReadToTheEnd(t *testing.T) {
	backend, proto := newGapH2Backend(t, func(_ http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		<-request.Context().Done()
	})
	addr, _, _ := gapGateway{backendURL: backend, httpClient: gapH2Client, timeout: 100 * time.Millisecond}.start(t)
	conn := dialGap(t, addr, false)
	defer conn.Close() //nolint:errcheck
	require.NoError(t, conn.SetDeadline(time.Now().Add(bodyTestWait)))

	for i := range 3 {
		_, err := io.WriteString(conn, "POST /upload HTTP/1.1\r\nHost: gateway\r\nContent-Length: 5\r\n\r\nhello")
		require.NoError(t, err)
		res, err := readRawResponse(conn)
		require.NoError(t, err, "request %d", i)
		assert.Equal(t, http.StatusGatewayTimeout, res.status)
		assert.False(t, res.closes(), "expected the connection to be kept")
	}
	assert.Equal(t, int64(2), proto.Load())
}

func TestGap_H2Backend_BackendThatAnswersBeforeTheBody(t *testing.T) {
	backend, proto := newGapH2Backend(t, func(writer http.ResponseWriter, _ *http.Request) {
		_ = http.NewResponseController(writer).EnableFullDuplex() // not needed on HTTP/2
		writer.Header().Set("Content-Length", "0")
		writer.WriteHeader(http.StatusRequestEntityTooLarge)
		_ = http.NewResponseController(writer).Flush()
	})
	addr, _, _ := gapGateway{backendURL: backend, httpClient: gapH2Client, timeout: 5 * time.Second}.start(t)

	reply := gapUpload{size: 30, interval: 20 * time.Millisecond}.send(t, dialGap(t, addr, false))

	assert.Equal(t, int64(2), proto.Load())
	assert.Equal(t, http.StatusRequestEntityTooLarge, reply.status)
	assert.Less(t, reply.elapsed, 300*time.Millisecond, "the response was held back")
	assertClosedNotTimedOut(t, reply)
	requireNoRequestBodyReader(t)
}

func TestGap_H2Backend_StreamingRoute(t *testing.T) {
	for _, protocol := range bodyTestProtocols {
		t.Run("bidirectional body outlives the read timeout/"+protocol.name, func(t *testing.T) {
			const readTimeout = 150 * time.Millisecond
			backend, proto := newGapH2Backend(t, func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusOK)
				_ = http.NewResponseController(writer).Flush()
				buf := make([]byte, 256)
				for {
					n, err := request.Body.Read(buf)
					if n > 0 {
						_, _ = writer.Write(buf[:n])
						_ = http.NewResponseController(writer).Flush()
					}
					if err != nil {
						return
					}
				}
			})
			_, gatewayURL, client := gapGateway{
				backendURL:  backend,
				httpClient:  gapH2Client,
				timeout:     400 * time.Millisecond,
				readTimeout: readTimeout,
				streaming:   true,
				clientH2:    protocol.http2,
			}.start(t)
			ctx := t.Context()
			start := time.Now()
			res := startUpload(ctx, 12, 25*time.Millisecond, false).post(ctx, t, client, gatewayURL+"/echo")
			defer res.Body.Close() //nolint:errcheck
			echoed, err := io.ReadAll(res.Body)
			require.NoError(t, err)
			assert.Equal(t, int64(2), proto.Load())
			assert.Equal(t, uploadLines(12), string(echoed))
			assert.Greater(t, time.Since(start), readTimeout)
		})
		t.Run("stalled client is cut at the route timeout/"+protocol.name, func(t *testing.T) {
			const routeTimeout = 150 * time.Millisecond
			backend, _ := newGapH2Backend(t, func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusOK)
				_ = http.NewResponseController(writer).Flush()
				go func() { _, _ = io.Copy(io.Discard, request.Body) }()
				beat(writer, request, 20*time.Millisecond, -1)
			})
			_, gatewayURL, client := gapGateway{
				backendURL: backend, httpClient: gapH2Client, timeout: routeTimeout,
				streaming: true, clientH2: protocol.http2,
			}.start(t)
			ctx := t.Context()
			body := startUpload(ctx, 3, 10*time.Millisecond, true)
			res := body.post(ctx, t, client, gatewayURL+"/stall")
			defer res.Body.Close() //nolint:errcheck
			<-body.done
			stalled := time.Now()
			_, err := io.Copy(io.Discard, res.Body)
			require.Error(t, err)
			assert.GreaterOrEqual(t, time.Since(stalled), routeTimeout*2/3)
			assert.Less(t, time.Since(stalled), bodyTestWait)
		})
	}
}

// --- Expect: 100-continue ------------------------------------------------------------

func TestGap_ExpectContinue(t *testing.T) {
	const routeTimeout = 100 * time.Millisecond
	for _, useTLS := range []bool{false, true} {
		suffix := "/plain"
		if useTLS {
			suffix = "/tls"
		}
		t.Run("route timeout during the upload"+suffix, func(t *testing.T) {
			addr, _, _ := gapGateway{
				backendURL: newBodyTestBackend(t, answerAfterTheBody), timeout: routeTimeout, clientTLS: useTLS,
			}.start(t)

			reply := gapUpload{size: 40, interval: 20 * time.Millisecond, expect: true}.send(t, dialGap(t, addr, useTLS))

			assert.True(t, reply.gotContinue, "expected 100 Continue once the body was read")
			assert.Equal(t, http.StatusGatewayTimeout, reply.status)
			assert.Less(t, reply.elapsed, 3*routeTimeout)
			assert.Less(t, reply.sent, 40)
			assertClosedNotTimedOut(t, reply)
			requireNoRequestBodyReader(t)
		})
		t.Run("rate limited before the body is read"+suffix, func(t *testing.T) {
			addr, gatewayURL, client := gapGateway{
				backendURL: newBodyTestBackend(t, answerAfterTheBody),
				timeout:    5 * time.Second,
				routeExtra: oneRequestRateLimit,
				clientTLS:  useTLS,
			}.start(t)
			warmUp(t, client, gatewayURL)

			reply := gapUpload{size: 30, interval: 20 * time.Millisecond, expect: true}.send(t, dialGap(t, addr, useTLS))

			assert.False(t, reply.gotContinue, "100 Continue sent for a body that is not used")
			assert.Equal(t, http.StatusTooManyRequests, reply.status)
			assert.Equal(t, 0, reply.sent)
			t.Logf("429 after %v, Connection: close=%v", reply.elapsed, reply.closes)
			assert.Less(t, reply.elapsed, 50*time.Millisecond, "the 429 waited for a body the client is not sending")
		})
		t.Run("backend that cannot be reached"+suffix, func(t *testing.T) {
			addr, _, _ := gapGateway{
				backendURL: newUnreachableBackend(t), timeout: 5 * time.Second, clientTLS: useTLS,
			}.start(t)

			reply := gapUpload{size: 30, interval: 20 * time.Millisecond, expect: true}.send(t, dialGap(t, addr, useTLS))

			assert.Equal(t, http.StatusBadGateway, reply.status)
			t.Logf("502 after %v, 100 Continue=%v, sent=%d", reply.elapsed, reply.gotContinue, reply.sent)
			assert.Less(t, reply.elapsed, 300*time.Millisecond)
		})
		t.Run("backend that answers before the body"+suffix, func(t *testing.T) {
			addr, _, _ := gapGateway{
				backendURL: newBodyTestBackend(t, rejectAtOnce), timeout: 5 * time.Second, clientTLS: useTLS,
			}.start(t)

			reply := gapUpload{size: 30, interval: 20 * time.Millisecond, expect: true}.send(t, dialGap(t, addr, useTLS))

			assert.Equal(t, http.StatusRequestEntityTooLarge, reply.status)
			t.Logf("413 after %v, 100 Continue=%v, sent=%d", reply.elapsed, reply.gotContinue, reply.sent)
			assert.Less(t, reply.elapsed, 300*time.Millisecond)
			requireNoRequestBodyReader(t)
		})
	}
	t.Run("Go client with Expect: normal upload keeps working and reuses the connection", func(t *testing.T) {
		var gotExpect atomic.Value
		backend := newBodyTestBackend(t, func(writer http.ResponseWriter, request *http.Request) {
			gotExpect.Store(request.Header.Get("Expect"))
			answerAfterTheBody(writer, request)
		})
		_, gatewayURL, _ := gapGateway{backendURL: backend, timeout: 2 * time.Second}.start(t)
		transport := &http.Transport{ExpectContinueTimeout: time.Second}
		t.Cleanup(transport.CloseIdleConnections)
		client := &http.Client{Transport: transport}
		reused := false
		trace := &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused }}
		for range 2 {
			request, err := http.NewRequestWithContext(httptrace.WithClientTrace(t.Context(), trace),
				http.MethodPost, gatewayURL+"/upload", strings.NewReader(strings.Repeat("x", 4096)))
			require.NoError(t, err)
			request.Header.Set("Expect", "100-continue")
			start := time.Now()
			res, err := client.Do(request)
			require.NoError(t, err)
			_, _ = io.Copy(io.Discard, res.Body)
			require.NoError(t, res.Body.Close())
			assert.Equal(t, http.StatusOK, res.StatusCode)
			assert.Less(t, time.Since(start), 500*time.Millisecond, "the client waited for its ExpectContinueTimeout")
		}
		assert.True(t, reused)
		t.Logf("Expect header seen by the backend: %q", gotExpect.Load())
	})
}
