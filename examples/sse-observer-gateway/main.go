// Command sse-observer-gateway demonstrates observing a streamed Server-Sent Events
// response chunk by chunk from a custom filter, without buffering it.
//
// It starts an in-process SSE backend on :8001 that emits a few token deltas and a final
// usage event, then boots the gateway (default port :8000) with a custom SSEUsageLogger
// filter. The filter registers a stream observer on the response body that reassembles
// SSE events from the raw chunks as they flow to the client and, when the stream ends,
// logs the event count, total bytes, elapsed time and the final usage event.
//
// Run it with:
//
//	go run . config.json
//
// then, in another terminal:
//
//	curl -N http://localhost:8000/
//
// The events arrive incrementally at curl; the gateway logs the stream summary once the
// stream completes.
package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/go-playground/validator/v10"
)

const (
	requiredArgsLen = 2
	backendAddr     = "localhost:8001"
)

func main() {
	if len(os.Args) < requiredArgsLen {
		log.Fatal("config file argument is required")
	}
	startSSEBackend(backendAddr)

	cfg := readConfigFile(os.Args[1])
	builder := bootstrap.NewOptionsBuilder(cfg)
	builder.WithCustomFilters(bootstrap.CustomFilter{
		Name:    sseUsageLoggerFilterName,
		Builder: newSSEUsageLoggerBuilder(),
	})
	server, err := bootstrap.Initialize(builder.Build())
	if err != nil {
		log.Fatalf("gateway initialization failed: %s", err)
	}
	log.Printf("gateway listening on :8000, SSE backend on %s", backendAddr)
	log.Fatal(server.ListenAndServe())
}

func readConfigFile(filename string) *config.Config {
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("gateway initialization failed: %s", err)
	}
	cfg, err := config.NewReaderJSON(validator.New()).Read(fileBytes)
	if err != nil {
		log.Fatalf("gateway initialization failed: %s", err)
	}
	return cfg
}

// startSSEBackend serves a Server-Sent Events stream that emits three token deltas with a
// pause between them and a final usage event, flushing each so the gateway forwards them
// incrementally.
func startSSEBackend(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)
		flusher, _ := writer.(http.Flusher)
		flush := func() {
			if flusher != nil {
				flusher.Flush()
			}
		}
		for i := 1; i <= 3; i++ {
			_, _ = fmt.Fprintf(writer, "data: {\"delta\":\"token-%d\"}\n\n", i)
			flush()
			time.Sleep(200 * time.Millisecond)
		}
		_, _ = fmt.Fprint(writer, "data: {\"usage\":{\"total_tokens\":42}}\n\n")
		flush()
	})
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("sse backend failed: %s", err)
		}
	}()
}

const sseUsageLoggerFilterName = "SSEUsageLogger"

// sseUsageLogger observes the streamed response of an SSE backend and logs a summary when
// the stream ends. It never buffers the body, so streaming to the client is unaffected.
type sseUsageLogger struct{}

func newSSEUsageLoggerBuilder() gateway.FilterBuilderFunc {
	return func(map[string]any) (gateway.Filter, error) {
		return sseUsageLogger{}, nil
	}
}

func (sseUsageLogger) Name() string { return sseUsageLoggerFilterName }

func (sseUsageLogger) PreProcess(*gateway.Context) error { return nil }

func (sseUsageLogger) PostProcess(ctx *gateway.Context) error {
	contentType, _, _ := strings.Cut(ctx.Response.Headers.Get("Content-Type"), ";")
	if !strings.EqualFold(strings.TrimSpace(contentType), "text/event-stream") {
		return nil
	}
	// Capture the logger by value, never the context: response-side observer callbacks
	// run on the handler goroutine while the context is still valid, but retaining the
	// *Context would be unsafe once it returns to the pool.
	logger := ctx.Logger
	start := time.Now()

	var (
		pending   []byte
		events    int
		lastEvent string
	)
	ctx.Response.BodyReader.ObserveStream(
		func(chunk []byte) {
			// Chunks are transport-sized, not whole events: accumulate and split on the
			// blank line that terminates each SSE event.
			pending = append(pending, chunk...)
			for {
				idx := bytes.Index(pending, []byte("\n\n"))
				if idx < 0 {
					break
				}
				if data, ok := dataPayload(pending[:idx]); ok {
					lastEvent = data
				}
				pending = pending[idx+2:]
				events++
			}
		},
		func(total int64, err error) {
			logger.Info("sse stream finished",
				"events", events,
				"bytes", total,
				"duration", time.Since(start).String(),
				"final_event", lastEvent,
				"error", err)
		},
	)
	return nil
}

// dataPayload returns the payload of the last "data:" line in an SSE event block.
func dataPayload(event []byte) (string, bool) {
	var payload string
	var found bool
	for _, line := range strings.Split(string(event), "\n") {
		if rest, ok := strings.CutPrefix(line, "data:"); ok {
			payload = strings.TrimSpace(rest)
			found = true
		}
	}
	return payload, found
}
