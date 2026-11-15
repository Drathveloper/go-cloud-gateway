package gatewayhandler

import (
	"context"
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
	body := trackRequestBody(writer, request)
	gwRequest := gateway.NewGatewayRequest(request)
	ctx, cancel := gateway.NewGatewayContext(request.Context(), route, gwRequest)
	defer gateway.ReleaseGatewayContext(ctx)
	defer cancel()
	defer gwRequest.BodyReader.Close() //nolint:errcheck
	defer body.release()
	if err := h.doWithRecover(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// The gateway abandons the body when the route times out, but on its
			// own goroutine, which may not have run yet, and a timeout may also
			// come from a filter. net/http would read what is left of the body
			// before it writes the response.
			body.Abandon()
		} else {
			body.giveUpAfter(unreadBodyGrace)
		}
		closeAfterAbandonedBody(writer, request, body)
		h.errHandler.Handle(ctx, err, writer)
		if ctx.Response != nil && ctx.Response.BodyReader != nil {
			_ = ctx.Response.BodyReader.Close()
		}
		return
	}
	// The timeout may have fired while the gateway was returning its response.
	closeAfterAbandonedBody(writer, request, body)
	if ctx.Route.Streaming {
		// The read side of what writeResponse does with the write deadline.
		body.renewReadDeadline(ctx.Route.Timeout)
	} else {
		// The backend answered without waiting for the end of the body.
		body.giveUpAfter(unreadBodyGrace)
	}
	h.writeResponse(writer, ctx)
}

// closeAfterAbandonedBody makes the server close the connection after the response when
// the body of the request was abandoned, and stops it from being abandoned later on: see
// requestBody.settle for why the connection cannot be kept. It must be called before the
// response is written.
//
// Only HTTP/1 needs it. An HTTP/2 stream has no connection of its own to spoil, and
// there the header would shut down the connection of every other stream.
func closeAfterAbandonedBody(writer http.ResponseWriter, request *http.Request, body *requestBody) {
	if body.settle() && request.ProtoMajor == 1 {
		writer.Header().Set("Connection", "close")
	}
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
	switch {
	case ctx.Route.Streaming:
		// The server write timeout is a deadline for the whole response and would
		// cut the stream: it is replaced by a deadline renewed on every write, which
		// still drops clients that stop reading. net/http clears the connection
		// write deadline once the handler returns, so it does not leak into the
		// next request of a kept-alive connection.
		dst = &immediateFlushWriter{
			dst:          writer,
			controller:   http.NewResponseController(writer),
			writeTimeout: ctx.Route.Timeout,
		}
	case isStreamingResponse(response):
		// Without per-write flushing, streamed events would sit in the server
		// output buffer until it fills.
		dst = &immediateFlushWriter{dst: writer, controller: http.NewResponseController(writer)}
	}
	if _, err := io.Copy(dst, response.BodyReader); err != nil {
		ctx.Logger.Warn("copying backend response to client failed", "error", err)
		// The status and part of the body may already be on the wire: abort the
		// connection so the client sees the truncation instead of a chunked
		// response that terminates cleanly and looks complete.
		panic(http.ErrAbortHandler)
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
type immediateFlushWriter struct {
	dst          io.Writer
	controller   *http.ResponseController
	writeTimeout time.Duration
}

func (w *immediateFlushWriter) Write(output []byte) (int, error) {
	if w.writeTimeout > 0 {
		// Not every response writer supports deadlines: without one the write is
		// only bounded by the server write timeout, as before.
		_ = w.controller.SetWriteDeadline(time.Now().Add(w.writeTimeout))
	}
	written, err := w.dst.Write(output)
	if err != nil {
		return written, err //nolint:wrapcheck
	}
	if err := w.controller.Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return written, err //nolint:wrapcheck
	}
	return written, nil
}
