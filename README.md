[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/Drathveloper/go-cloud-gateway)
[![codecov](https://codecov.io/gh/Drathveloper/go-cloud-gateway/branch/master/graph/badge.svg?token=IKVZK1594Y)](https://codecov.io/gh/Drathveloper/go-cloud-gateway)

# go-cloud-gateway

**go-cloud-gateway** is a lightweight, configuration-driven API gateway written in Go. It functions as a reverse proxy, routing HTTP requests to backend services based on configurable predicates and applying request/response transformations through a pluggable filter system.

## Features

* **Configurable Routing**: Define routes using predicates based on path, method, and host matching.
* **Pluggable Filters**: Apply request and response transformations through a modular filter pipeline.
* **Flexible Configuration**: Support for both YAML and JSON configuration formats with built-in validation.
* **Extensible Architecture**: Utilize factory patterns for dynamic creation of filters and predicates.
* **Lightweight and Efficient**: Designed for minimal overhead and high performance.

## Architecture Overview

The gateway's architecture is modular, comprising three main subsystems:

1. **Configuration System**: Loads and validates gateway configurations, including routes, filters, and global settings.
2. **Request Processing Pipeline**: Processes incoming HTTP requests through a series of filters before forwarding them to the appropriate backend service.
3. **Filter and Predicate Ecosystem**: Provides a framework for defining and applying filters and predicates to control request routing and transformation.

## Getting Started

### Prerequisites

* Go 1.24+ installed on your system.

### Installation

- Clone the repository:

   ```bash
   git clone https://github.com/drathveloper/go-cloud-gateway.git
   cd go-cloud-gateway
   ```

- Execute tests:

   ```bash
   make test
   ```

- Execute tests with coverage:

   ```bash
   make test-cover
   ```
   ```bash
   make test-html
   ```
  
- Execute lint:

   ```bash
   make lint
   ```

### Configuration

The gateway can be configured using either YAML or JSON files. A basic YAML configuration example:

```yaml
gateway:
  routes:
    - id: example-route
      uri: http://localhost:8080
      predicates:
        - name: Path
          args:
            patterns:
              - /api/v1/*
      filters:
        - name: AddRequestHeader
          args:
            name: X-Request-ID
            value: uuid
  global-filters:
    - name: RequestResponseLogger
  global-timeout: 30s
```

* **Routes**: Define individual routes with associated predicates and filters.
* **Global Filters**: Filters applied to all incoming requests.
* **Settings**: Global settings such as timeouts and client configurations.

## Extending the Gateway

The gateway's architecture allows for easy extension:

* **Custom Filters**: Implement the `Filter` interface and register your filter using the `FilterFactory`.
* **Custom Predicates**: Implement the `Predicate` interface and register your predicate using the `PredicateFactory`.

This design enables dynamic creation and application of filters and predicates based on configuration, promoting flexibility and testability.

### Observing streamed bodies

Post-process filters run once, after the backend headers arrive but **before** the body
streams to the client. The only way for a filter to read the body at that point is
`Capture`, which buffers the whole body in memory and defeats streaming — unacceptable for
Server-Sent Events or any long-lived response.

`ReplayableBody.ObserveStream` is the streaming-friendly alternative. It registers
callbacks invoked as the body flows from its source, without buffering it, so a filter can
meter token usage, count SSE events, or measure stream size and duration while the client
still receives the bytes incrementally:

```go
func (f *SSEUsageLogger) PostProcess(ctx *gateway.Context) error {
    // Capture the logger by value, never the *Context: response-side callbacks run on
    // the handler goroutine while the context is valid, but it returns to a pool after.
    logger := ctx.Logger
    start := time.Now()
    var events int
    ctx.Response.BodyReader.ObserveStream(
        func(chunk []byte) { events += bytes.Count(chunk, []byte("\n\n")) },
        func(total int64, err error) {
            logger.Info("sse stream finished", "events", events, "bytes", total,
                "duration", time.Since(start), "error", err)
        },
    )
    return nil
}
```

Contract:

* **The chunk slice is only valid during the `onChunk` call** — it aliases a reused read
  buffer. Copy anything that must outlive the callback.
* **Chunks are transport-sized reads, not application messages.** An SSE consumer must
  reassemble events from the chunk stream itself.
* **`onDone` fires exactly once**: a `nil` error on clean EOF, the read error on a
  mid-stream failure, or `ErrStreamTruncated` when the body is closed before EOF (client
  disconnect, pipeline error, or a discarded response).
* **Request-side callbacks may run on a transport goroutine.** Observers on the request
  body must be safe for that and must not retain the `*Context`.

See [`examples/sse-observer-gateway`](examples/sse-observer-gateway) for a runnable
gateway that logs the event count, byte total, duration and final usage event of a
streamed SSE backend.

## Dependencies

Key external libraries are used:

* [`github.com/go-playground/validator/v10`](https://pkg.go.dev/github.com/go-playground/validator/v10): For configuration validation.
* [`github.com/stretchr/testify`](https://pkg.go.dev/github.com/stretchr/testify): For additional testing utilities.
* [`golang.org/x/net`](https://pkg.go.dev/golang.org/x/net): For http2 networking package.
* [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3): For parsing YAML configuration files.

## Contributing

Contributions are welcome! Please fork the repository and submit a pull request with your enhancements or bug fixes.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

For more detailed information, refer to the [DeepWiki documentation](https://deepwiki.com/Drathveloper/go-cloud-gateway).

---


