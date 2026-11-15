package gatewayhandler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// ErrRouteNotFound is the error returned when no route matched the request.
var ErrRouteNotFound = errors.New("route not found")

// ErrPanic is the error a recovered panic from the gateway pipeline is mapped to.
var ErrPanic = errors.New("panic while handling gateway request")

// Gateway is the interface for the gateway.
// It is used to handle the request and return the response.
// The gateway context is used to store the request, response, contextual logging and other information.
type Gateway interface {
	Do(ctx *gateway.Context) error
}

// GatewayHandler is the handler for the gateway.
// It is used to handle the request, search the appropriate route and handle the gateway the response from context.
type GatewayHandler struct {
	gateway    Gateway
	errHandler ErrorHandler
	notFound   http.Handler
	routes     gateway.Routes
}

// Option configures a GatewayHandler.
type Option func(*GatewayHandler)

// WithNotFoundHandler sets the handler invoked when no route matches the request.
//
// Route not found is a routing outcome, not a pipeline error: there is no route and
// therefore no gateway context, so it deliberately does not go through the ErrorHandler.
// The default handler replies 404 Not Found with a plain text body.
func WithNotFoundHandler(handler http.Handler) Option {
	return func(h *GatewayHandler) {
		if handler != nil {
			h.notFound = handler
		}
	}
}

// NewGatewayHandler creates a new gateway handler.
func NewGatewayHandler(
	gateway Gateway,
	routes gateway.Routes,
	errHandler ErrorHandler,
	opts ...Option) *GatewayHandler {
	handler := &GatewayHandler{
		gateway:    gateway,
		routes:     routes,
		errHandler: errHandler,
		notFound: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			http.Error(writer, ErrRouteNotFound.Error(), http.StatusNotFound)
		}),
	}
	for _, opt := range opts {
		opt(handler)
	}
	return handler
}

// ServeHTTP is the entrypoint for all requests to the gateway.
func (h *GatewayHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	route := h.routes.FindMatching(request)
	if route == nil {
		h.notFound.ServeHTTP(writer, request)
		return
	}
	shared.SetXForwardedHeaders(request)
	gwRequest := gateway.NewGatewayRequest(request)
	ctx, cancel := gateway.NewGatewayContext(request.Context(), route, gwRequest)
	defer gateway.ReleaseGatewayContext(ctx)
	defer cancel()
	defer gwRequest.BodyReader.Close() //nolint:errcheck
	if err := h.doWithRecover(ctx); err != nil {
		h.errHandler.Handle(ctx, err, writer)
		if ctx.Response != nil && ctx.Response.BodyReader != nil {
			_ = ctx.Response.BodyReader.Close()
		}
		return
	}
	h.writeResponse(writer, ctx)
}

// doWithRecover runs the gateway pipeline converting panics into errors: filters are a
// public extension point and a panicking filter must answer the client with an error
// instead of killing the connection without a response. Nothing has been written to the
// client at this stage, so the error handler can still produce a full response.
// http.ErrAbortHandler is passed through untouched for callers that abort on purpose.
func (h *GatewayHandler) doWithRecover(ctx *gateway.Context) (err error) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		if recovered == http.ErrAbortHandler { //nolint:errorlint,err113 // sentinel comparison per net/http contract
			panic(recovered)
		}
		// The stack is only available here: the error handler just sees ErrPanic.
		ctx.Logger.Error("panic while handling gateway request",
			"panic", recovered,
			"stack", string(debug.Stack()))
		err = fmt.Errorf("%w: %v", ErrPanic, recovered)
	}()
	return h.gateway.Do(ctx) //nolint:wrapcheck
}

func (h *GatewayHandler) writeResponse(writer http.ResponseWriter, ctx *gateway.Context) {
	response := ctx.Response
	defer response.BodyReader.Close() //nolint:errcheck
	shared.RemoveHopByHopHeaders(response.Headers)
	shared.WriteHeader(writer, response.Headers)
	if length := response.BodyReader.Len(); length >= 0 {
		writer.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	} else {
		writer.Header().Del("Content-Length")
	}
	writer.WriteHeader(response.Status)
	dst := io.Writer(writer)
	stream := newStreamWriter(writer, ctx)
	if stream != nil {
		dst = stream
		// The headers of a stream are sent before its first event, which may take a
		// while, as the net/http/httputil reverse proxy does: clients such as
		// EventSource report the stream as open when they get them.
		if err := stream.flush(); err != nil {
			ctx.Logger.Warn("sending response headers to client failed", "error", err)
			panic(http.ErrAbortHandler)
		}
	}
	if _, err := io.Copy(dst, response.BodyReader); err != nil {
		ctx.Logger.Warn("copying backend response to client failed", "error", err)
		// The status and part of the body may already be on the wire: abort the
		// connection so the client sees the truncation instead of a chunked
		// response that terminates cleanly and looks complete.
		panic(http.ErrAbortHandler)
	}
	if stream != nil {
		stream.finish()
	}
}

// newStreamWriter returns the writer of a response that must reach the client as it is
// produced, or nil when the response can be buffered by the server as usual.
func newStreamWriter(writer http.ResponseWriter, ctx *gateway.Context) *immediateFlushWriter {
	switch {
	case ctx.Route.Streaming:
		// The server write timeout is a deadline for the whole response and would
		// cut the stream: it is replaced by a deadline of its own for every write,
		// which still drops clients that stop reading.
		return &immediateFlushWriter{
			dst:          writer,
			controller:   http.NewResponseController(writer),
			writeTimeout: ctx.Route.Timeout,
		}
	case isStreamingResponse(ctx.Response):
		// Without per-write flushing, streamed events would sit in the server
		// output buffer until it fills.
		return &immediateFlushWriter{dst: writer, controller: http.NewResponseController(writer)}
	default:
		return nil
	}
}

// isStreamingResponse reports whether the response must reach the client as it is
// produced: responses of unknown length and server-sent event streams. It mirrors
// the flush heuristic of the net/http/httputil reverse proxy.
func isStreamingResponse(response *gateway.Response) bool {
	if response.BodyReader.Len() == -1 {
		return true
	}
	contentType, _, _ := strings.Cut(response.Headers.Get("Content-Type"), ";")
	return strings.EqualFold(textproto.TrimString(contentType), "text/event-stream")
}

// immediateFlushWriter flushes the response writer after every write so streamed
// data is put on the wire as it is produced. A positive writeTimeout additionally
// gives every write, flush included, its own deadline.
//
// The deadline only stands while a write is in progress. It is cleared once the write
// is done, because a write deadline is not the same thing on every protocol: on HTTP/1
// it only matters to the writes that happen before it, but on HTTP/2 it is a timer
// that resets the stream when it fires, whether or not anything is being written. Left
// in place, it would cut a stream that stays quiet for longer than the timeout.
type immediateFlushWriter struct {
	dst          io.Writer
	controller   *http.ResponseController
	writeTimeout time.Duration
}

func (w *immediateFlushWriter) Write(output []byte) (int, error) {
	w.setWriteDeadline()
	defer w.clearWriteDeadline()
	written, err := w.dst.Write(output)
	if err != nil {
		return written, err //nolint:wrapcheck
	}
	if err := w.controller.Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return written, err //nolint:wrapcheck
	}
	return written, nil
}

// flush puts on the wire what has been written so far, the response headers included,
// under a deadline of its own.
func (w *immediateFlushWriter) flush() error {
	w.setWriteDeadline()
	defer w.clearWriteDeadline()
	if err := w.controller.Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err //nolint:wrapcheck
	}
	return nil
}

// finish gives a deadline to what the server still writes once the handler returns: the
// end of the response, which is the last chunk on HTTP/1 and the end of the stream on
// HTTP/2. The server would otherwise write it without any deadline, or, were the deadline
// of the last write left in place, under one that expired if the response ended more than
// a timeout after that write, which makes the client see a truncated response.
//
// net/http clears the write deadline of the connection once the handler has returned
// and the response is finished, and an HTTP/2 stream stops its timer when it closes, so
// the deadline does not outlive the response.
func (w *immediateFlushWriter) finish() {
	w.setWriteDeadline()
}

func (w *immediateFlushWriter) setWriteDeadline() {
	if w.writeTimeout > 0 {
		// Not every response writer supports deadlines: without one the write is
		// only bounded by the server write timeout, as before.
		_ = w.controller.SetWriteDeadline(time.Now().Add(w.writeTimeout))
	}
}

func (w *immediateFlushWriter) clearWriteDeadline() {
	if w.writeTimeout > 0 {
		_ = w.controller.SetWriteDeadline(time.Time{})
	}
}
