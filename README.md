[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/Drathveloper/go-cloud-gateway)
[![codecov](https://codecov.io/gh/Drathveloper/go-cloud-gateway/branch/master/graph/badge.svg?token=IKVZK1594Y)](https://codecov.io/gh/Drathveloper/go-cloud-gateway)

# go-cloud-gateway

**go-cloud-gateway** is a lightweight, configuration-driven API gateway written in Go. It functions as a reverse proxy, routing HTTP requests to backend services based on configurable predicates and applying request/response transformations through a pluggable filter system.

## Features

* **Configurable Routing**: Routes are selected by predicates on the path, host, method, headers, query
  parameters, cookies and the current time.
* **Filters**: Add, set and remove request and response headers, rewrite the path, log requests and responses,
  and rate limit. Filters run per route or for every route.
* **Timeouts and circuit breaker**: Every route has a timeout and can have its own circuit breaker.
* **Streaming**: Long-lived responses such as Server-Sent Events are forwarded as they are produced, and filters
  can observe them without buffering.
* **Flexible Configuration**: YAML and JSON configuration with validation when it is read.
* **Extensible**: Custom filters, predicates, error handler, not found handler and extra HTTP handlers are
  registered when the gateway is initialized.
* **Few dependencies**: Built on `net/http`. The only runtime dependencies are the configuration validator and
  the YAML parser.

## Architecture Overview

The gateway's architecture is modular, comprising three main subsystems:

1. **Configuration System**: Loads and validates gateway configurations, including routes, filters, and global settings.
2. **Request Processing Pipeline**: Processes incoming HTTP requests through a series of filters before forwarding them to the appropriate backend service.
3. **Filter and Predicate Ecosystem**: Provides a framework for defining and applying filters and predicates to control request routing and transformation.

## Getting Started

### Prerequisites

* Go 1.27+ installed on your system.

### Installation

- Add the library to your module:

   ```bash
   go get github.com/drathveloper/go-cloud-gateway
   ```

- Or clone the repository to work on it. The `make` targets come from the `.parent` submodule:

   ```bash
   git clone --recurse-submodules https://github.com/drathveloper/go-cloud-gateway.git
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

### Usage

The gateway is a library: read a configuration, initialize the gateway and start the returned `*http.Server`.

```go
package main

import (
    "log"
    "os"

    "github.com/go-playground/validator/v10"

    "github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
    "github.com/drathveloper/go-cloud-gateway/pkg/config"
)

func main() {
    content, err := os.ReadFile("gateway.yaml")
    if err != nil {
        log.Fatal(err)
    }
    cfg, err := config.NewReaderYAML(validator.New()).Read(content)
    if err != nil {
        log.Fatal(err)
    }
    server, err := bootstrap.Initialize(bootstrap.NewOptionsBuilder(cfg).Build())
    if err != nil {
        log.Fatal(err)
    }
    log.Fatal(server.ListenAndServe())
}
```

`config.NewReaderJSON` reads the same configuration written in JSON, with the same keys.
[`examples/minimal-gateway`](examples/minimal-gateway) is a runnable gateway. The server listens on port 8000
unless [the bootstrap options](#bootstrap-options) say otherwise.

### Configuration built in code

A program that reads no file can build the `config.Config` itself. The field names are the configuration keys
in camel case, and a duration is a `config.Duration`, which wraps a `time.Duration`:

```go
cfg := &config.Config{
    Gateway: config.Gateway{
        GlobalTimeout: config.Duration{Duration: 30 * time.Second},
        Routes: []config.Route{
            {
                ID:  "example-route",
                URI: "http://localhost:8080",
                Predicates: []config.ParameterizedItem{
                    {Name: "Path", Args: map[string]any{"patterns": []any{"/api/v1/*"}}},
                },
            },
        },
    },
}
server, err := bootstrap.Initialize(bootstrap.NewOptionsBuilder(cfg).Build())
```

`bootstrap.Initialize` validates the configuration with `config.Validate` and returns an error when it is not
valid, with one line per broken rule. The rules are the ones the readers apply, which makes everything this
document says about a required key or a range true for the matching field. `config.Validate(cfg)` can also be
called on its own. Some rules are easy to miss in code, where a field that is left out is a zero:

* `Gateway.Routes` needs at least one route, and each route an `ID` and a `URI`.
* When `HTTPClient.Pool` is set, its five durations are pointers that cannot be nil, although they can point
  to a zero duration, and its three counters cannot be zero. Leave `Pool` nil to get the defaults.
* When `HTTPClient.MTLS` is set, `Enabled` is a pointer that cannot be nil.
* When `CircuitBreaker.Enabled` is `true`, no other field of the circuit breaker can be zero.

## Configuration

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
            name: X-Gateway
            value: go-cloud-gateway
  global-filters:
    - name: RequestResponseLogger
  global-timeout: 30s
```

Every YAML example in this document is a complete configuration and is read and built by a test
(`pkg/bootstrap/readme_examples_test.go`). Every Go snippet is compiled by another one
(`pkg/bootstrap/readme_go_snippets_test.go`).

Settings under `gateway`:

| Key              | Required | Description                                                                         |
|------------------|----------|-------------------------------------------------------------------------------------|
| `routes`         | yes      | List of routes, at least one. They are evaluated in order.                          |
| `global-filters` | no       | Filters applied to every route, before the filters of the route.                    |
| `global-timeout` | no       | Timeout of the routes that do not set their own. Defaults to 10 seconds.            |
| `httpclient`     | no       | Settings of the client that calls the backends. See [HTTP client](#http-client).    |

Settings of each route:

| Key               | Required | Description                                                                                    |
|-------------------|----------|------------------------------------------------------------------------------------------------|
| `id`              | yes      | Name of the route. It is the `route` attribute of its log lines and the name of its breaker.   |
| `uri`             | yes      | Backend, as an `http` or `https` URL with a host, such as `http://localhost:8080`.             |
| `predicates`      | no       | Conditions a request must meet to use the route. See [Predicates](#predicates).                |
| `filters`         | no       | Filters applied to the route. See [Filters](#filters).                                         |
| `timeout`         | no       | Timeout of the route. See [Timeouts and streaming routes](#timeouts-and-streaming-routes).     |
| `streaming`       | no       | `true` for routes with long-lived responses. Defaults to `false`.                              |
| `circuit-breaker` | no       | Circuit breaker of the route. See [Circuit breaker](#circuit-breaker).                         |

A request goes to the first route, in configuration order, whose predicates all match. A route without
predicates matches every request. A request that matches no route is answered with `404 Not Found`.

Only the scheme and the host of `uri` are used. The path and the query string sent to the backend are the ones
of the request (after any `RewritePath` filter). A `uri` with a path, a query string, a fragment or user
info is rejected when the gateway starts; use a `RewritePath` filter to add a prefix to the path.

Durations are strings with a unit, such as `30s`, `1m30s` or `250ms`. A bare number like
`30` is rejected when the configuration is read.

### Predicates

| Predicate | Arguments                    | Matches when                                                             |
|-----------|------------------------------|--------------------------------------------------------------------------|
| `Path`    | `patterns` (list)            | The request path matches at least one pattern.                           |
| `Host`    | `patterns` (list)            | The `Host` header matches at least one pattern.                          |
| `Method`  | `methods` (list)             | The request method is in the list. The methods can be written in any case. |
| `Header`  | `name`, `regexp` (optional)  | The request has the header and, with `regexp`, one of its values matches it. |
| `Query`   | `name`, `regexp` (optional)  | The request has the query parameter and, with `regexp`, one of its values matches it. |
| `Cookie`  | `name`, `regexp` (optional)  | The request has the cookie and, with `regexp`, one of its values matches it. |
| `After`   | `datetime`                   | The current time is after `datetime`.                                    |
| `Before`  | `datetime`                   | The current time is before `datetime`.                                   |
| `Between` | `start`, `end`               | The current time is after `start` and before `end`.                      |

* All the predicates of a route must match. Within `Path`, `Host` and `Method`, one item of the list is enough.
* Without `regexp`, `Header`, `Query` and `Cookie` only check presence: a header, parameter or cookie with an
  empty value is present (`?flag` and `?flag=` both match `Query` with `name: flag`).
* `regexp` uses the Go [regular expression syntax](https://pkg.go.dev/regexp/syntax) and matches anywhere in
  the value. Use `^` and `$` to match the whole value. An empty value can match it.
* Datetimes are [RFC 3339](https://www.rfc-editor.org/rfc/rfc3339) timestamps, such as `2030-01-01T00:00:00Z`.

```yaml
gateway:
  routes:
    - id: orders-beta
      uri: http://localhost:8081
      predicates:
        - name: Path
          args:
            patterns:
              - /orders/**
        - name: Host
          args:
            patterns:
              - "*.example.org"
        - name: Method
          args:
            methods:
              - GET
              - POST
        - name: Header
          args:
            name: X-Beta
        - name: Query
          args:
            name: version
            regexp: ^v[0-9]+$
        - name: Cookie
          args:
            name: session
        - name: Between
          args:
            start: "2026-01-01T00:00:00Z"
            end: "2030-01-01T00:00:00Z"
    - id: orders
      uri: http://localhost:8080
      predicates:
        - name: Path
          args:
            patterns:
              - /orders/**
```

#### Path and Host patterns

The `Path` predicate matches the request path segment by segment:

* `**` matches any number of segments, including none: `/api/**/users/*` matches
  `/api/users/7` and `/api/v1/tenants/users/7`.
* `*` matches any number of characters within a segment and `?` exactly one.

The `Host` predicate matches the `Host` header, case-insensitively:

* `*` matches one or more characters within a label, `**` across labels and `?` exactly one
  character. Everything else is literal.
* The port is ignored unless the pattern has one: `example.org` matches
  `example.org:8080`, while `example.org:8080` only matches that port.

### Filters

| Filter                  | Arguments                 | Effect                                                               |
|-------------------------|---------------------------|----------------------------------------------------------------------|
| `AddRequestHeader`      | `name`, `value`           | Adds a value to a header of the request sent to the backend, keeping the values it already has. |
| `SetRequestHeader`      | `name`, `value`           | Sets a header of the request sent to the backend, replacing its values. |
| `RemoveRequestHeader`   | `name`                    | Removes a header from the request sent to the backend.               |
| `AddResponseHeader`     | `name`, `value`           | Adds a value to a header of the response sent to the client, keeping the values it already has. |
| `SetResponseHeader`     | `name`, `value`           | Sets a header of the response sent to the client, replacing its values. |
| `RemoveResponseHeader`  | `name`                    | Removes a header from the response sent to the client.               |
| `RewritePath`           | `regexp`, `replacement`   | Rewrites the path of the request sent to the backend.                |
| `RequestResponseLogger` | all optional, see below   | Logs every request and response.                                     |
| `RateLimit`             | see [Rate limiting](#rate-limiting) | Rejects the requests over a rate with `429 Too Many Requests`. |

* On the way to the backend the global filters run first, then the filters of the route, each list in
  configuration order. On the way back they run in the reverse order.
* **Global filters are instantiated once per route.** A filter that keeps state keeps it per route: a
  `RateLimit` in `global-filters` limits each route separately, not the gateway as a whole.
* Header values are literal strings. There are no generated or computed values: `value: uuid` sends the
  text `uuid`.
* `RewritePath` replaces every match of `regexp` in the path with `replacement`, and leaves the path alone
  when there is no match. `replacement` can refer to capture groups as `$1` or `${name}` (`$\{name}` is also
  accepted). Both use the Go [regular expression syntax](https://pkg.go.dev/regexp/syntax).

Arguments of `RequestResponseLogger`:

| Argument         | Default | Description                                                                          |
|------------------|---------|--------------------------------------------------------------------------------------|
| `level`          | `info`  | Level of the log lines: `debug`, `info`, `warn` or `error`, in any case. Any other value is rejected when the gateway starts. |
| `log-bodies`     | `true`  | Whether the bodies are logged. With `false` no body is buffered or copied.           |
| `max-body-bytes` | `65536` | Most body bytes logged per request and per response. A negative value means no limit. |
| `redact-headers` | none    | List of header names whose values are redacted, besides the ones that always are.    |
| `redact-query-params` | none | List of query parameter names whose values are redacted in the logged URL, besides the ones that always are. |

The request body is buffered in memory to be logged, up to `max-body-bytes`: a bigger request body is
forwarded untouched and logged without its content. The response body is not held back: it streams to the
client while its first `max-body-bytes` bytes are kept for the log line, which is written, with the total
size of the body, once the body has been sent.

`RequestResponseLogger` logs the values of `Authorization`, `Proxy-Authorization`,
`Cookie`, `Set-Cookie` and `X-Api-Key` as `[REDACTED]`. Its `redact-headers` argument
adds more header names to that list.

The URL of the request is logged with the credentials it carries replaced the same way:

* The values of the query parameters `access_token`, `id_token`, `refresh_token`, `token`, `api_key`,
  `apikey`, `password`, `secret`, `client_secret` and `signature`. The `redact-query-params` argument adds
  more names to that list. `GET /callback?state=xyz&access_token=abc` is logged as
  `GET /callback?state=xyz&access_token=[REDACTED]`.
* The user info, which a request only has when its request line holds an absolute URL:
  `http://user:password@example.org/path` is logged as `http://[REDACTED]@example.org/path`.

How the query string is redacted:

* Only the log line changes. The backend receives the query string as the client sent it.
* A name matches whatever the case of its ASCII letters, and it is compared decoded: `Access_Token=abc` and
  `access%5Ftoken=abc` are both redacted. The names of `redact-query-params` are written decoded: `api key`,
  not `api%20key` or `api+key`.
* The whole name has to match: `token_type` and `mytoken` are logged as they are.
* Every value of a repeated parameter is redacted, and so is a parameter that follows a `;` instead of a `&`
  (`?a=1;token=abc`).
* The rest of the URL is logged as it was received: the name of the redacted parameter, the order of the
  parameters, their encoding and the ones that are malformed. A sensitive parameter with no value or with an
  empty one (`?token`, `?token=`) is also logged as it is.
* The path and the bodies are not redacted: a credential sent in either of them is logged.

In code, `WithRedactedHeaders` and `WithRedactedQueryParams` return a copy of the filter with more names.

```yaml
gateway:
  global-filters:
    - name: RequestResponseLogger
      args:
        level: debug
        log-bodies: true
        max-body-bytes: 4096
        redact-headers:
          - X-Internal-Token
        redact-query-params:
          - session_id
  routes:
    - id: users
      uri: http://localhost:8080
      predicates:
        - name: Path
          args:
            patterns:
              - /api/v1/users/**
      filters:
        - name: RewritePath
          args:
            regexp: ^/api/v1/(.*)$
            replacement: /$1
        - name: SetRequestHeader
          args:
            name: X-Gateway
            value: go-cloud-gateway
        - name: RemoveRequestHeader
          args:
            name: X-Debug
        - name: AddResponseHeader
          args:
            name: Cache-Control
            value: no-store
        - name: RemoveResponseHeader
          args:
            name: Server
```

### Rate limiting

The `RateLimit` filter gives every key a token bucket: each request takes one token, and a request that
finds the bucket empty is answered with `429 Too Many Requests`.

| Argument      | Required            | Description                                                           |
|---------------|---------------------|-----------------------------------------------------------------------|
| `type`        | yes                 | Limiter. The only built-in one is `in-memory`.                        |
| `key`         | yes                 | What the requests are grouped by. See the table below.                |
| `rate`        | with `in-memory`    | Tokens added to each bucket per second. An integer greater than zero. |
| `burst`       | with `in-memory`    | Size of each bucket. An integer greater than zero.                    |
| `header-name` | with `key: header`  | Name of the header.                                                   |
| `query-param` | with `key: query`   | Name of the query parameter.                                          |

| Key           | Requests are grouped by                                                                    |
|---------------|--------------------------------------------------------------------------------------------|
| `ip`          | The client address: the first entry of `X-Forwarded-For` when it is an IP, else `X-Real-Ip` when it is an IP, else the address of the connection. |
| `path`        | The request path.                                                                          |
| `path-method` | The request method and path.                                                               |
| `header`      | The value of the header named by `header-name`.                                            |
| `query`       | The value of the query parameter named by `query-param`.                                   |

* The `in-memory` limiter lives in the gateway process: several gateway instances do not share it.
* Each `RateLimit` entry has its own limiter, and global filters are instantiated once per route. A
  `RateLimit` in `global-filters` therefore limits each route separately, not the gateway as a whole.
* `429` responses of the `in-memory` limiter carry a `Retry-After` header with the seconds, rounded up, until
  the bucket of the key has a token again.
* `X-Forwarded-For` and `X-Real-Ip` are set by the client unless a trusted proxy in front of the gateway
  overwrites them, so without such a proxy a client can choose its own `ip` key.
* The requests that lack the header or the query parameter of a `header` or `query` key all get the same
  (empty) key, so they share one bucket and are limited together, whichever client sends them.

```yaml
gateway:
  routes:
    - id: search
      uri: http://localhost:8080
      predicates:
        - name: Path
          args:
            patterns:
              - /search/**
      filters:
        - name: RateLimit
          args:
            type: in-memory
            key: ip
            rate: 10
            burst: 20
    - id: partners
      uri: http://localhost:8081
      predicates:
        - name: Path
          args:
            patterns:
              - /partners/**
      filters:
        - name: RateLimit
          args:
            type: in-memory
            key: header
            header-name: X-Api-Key
            rate: 100
            burst: 100
```

### Circuit breaker

A route with a circuit breaker stops calling its backend while it is failing. The breaker counts a failure
for every backend response with a status of 500 or more, and for every request that fails or times out. A
request cancelled by the client is not a failure. A `5xx` response is still sent to the client as it is.

| Setting                       | Description                                                                        |
|-------------------------------|------------------------------------------------------------------------------------|
| `enabled`                     | `true` to give the route a circuit breaker. Defaults to `false`.                   |
| `interval`                    | While the breaker is closed, its counters are reset every `interval`.              |
| `min-requests-threshold`      | Requests the breaker must have counted before it can open, from 1 to 4294967295.   |
| `failure-rate-threshold`      | Percentage of failed requests at which the breaker opens, from 1 to 100.           |
| `wait-duration-in-open-state` | Time the breaker stays open before it lets requests through again (half-open).     |
| `num-allowed-half-open-calls` | Requests let through while half-open, from 1 to 4294967295. The breaker closes when that many succeed in a row and opens again on the first failure. |

All the settings are required when `enabled` is `true`, and none can be zero. A negative duration and a
number out of its range are rejected when the gateway starts, also when `enabled` is `false`. While the
breaker is open, and for the requests over the allowed ones while it is half-open, the gateway answers
`503 Service Unavailable` without calling the backend.

```yaml
gateway:
  routes:
    - id: payments
      uri: http://localhost:8080
      predicates:
        - name: Path
          args:
            patterns:
              - /payments/**
      circuit-breaker:
        enabled: true
        interval: 1m
        min-requests-threshold: 20
        failure-rate-threshold: 50
        wait-duration-in-open-state: 30s
        num-allowed-half-open-calls: 5
```

### Timeouts and streaming routes

Every route has a timeout: its own `timeout`, else `global-timeout`, else 10 seconds. It
bounds the **whole exchange**, response body included, and the gateway answers
`504 Gateway Timeout` when the backend does not reply in time.

That is the wrong shape for long-lived responses such as Server-Sent Events, which would
be cut when the timeout expires. Mark those routes as streaming:

```yaml
gateway:
  routes:
    - id: events
      uri: http://localhost:8080
      timeout: 5s
      streaming: true
```

On a streaming route the timeout only bounds:

* the time until the response is ready to be sent (pre-process filters, backend response
  headers and post-process filters): past it the gateway still answers `504`;
* each individual write to the client, so a client that stops reading is dropped.

Once the response starts, the stream lasts as long as the backend keeps it open or until
the client disconnects. The response headers are sent as soon as the backend has answered,
without waiting for the first data, and every write is flushed immediately. The timeout does
not bound the time between two writes nor the time the stream takes to end: a stream can
stay quiet for longer than the timeout, over HTTP/1.1 and over HTTP/2.

The server `ReadTimeout` and `WriteTimeout` (`bootstrap.ServerOpts`) are also deadlines,
for the whole request and for the whole response, and start counting before the route
timeout. `bootstrap.Initialize` raises each of them to the longest route timeout plus 5
seconds when it is shorter than that. Otherwise an expired read timeout would cut a request
body that is still being sent to the backend, and an expired write timeout would close the
connection before the gateway can answer `504`. A zero or negative value, which disables
the timeout, is kept. Streaming routes are not subject to the write timeout, but their
request body is still bound by the read timeout.

### HTTP client

The `httpclient` section configures the client that calls the backends. The whole section is optional,
and so is each of its keys.

| Key                   | Default   | Description                                                                    |
|-----------------------|-----------|--------------------------------------------------------------------------------|
| `enable-http2`        | `false`   | Use HTTP/2 with the backends that support it over TLS. Backends with an `http` URI are always called with HTTP/1.1. |
| `disable-compression` | `false`   | By default the client asks the backend for gzip when the request has no `Accept-Encoding` header, and decompresses those responses itself. `true` forwards the backend responses as they are encoded. |
| `insecure-tls-verify` | `false`   | `true` skips the verification of the backend certificates.                     |
| `pool`                | see below | Connection pool.                                                               |
| `mtls`                | none      | Client certificate presented to the backends.                                  |

Settings under `pool`. When the section is present all of them are required, and the counters must be
greater than zero:

| Key                       | Default | Description                                                              |
|---------------------------|---------|--------------------------------------------------------------------------|
| `timeout`                 | `10s`   | Time to establish a connection. It does not bound the request.           |
| `keep-alive`              | `30s`   | Interval of the TCP keep-alive probes.                                   |
| `idle-conn-timeout`       | `60s`   | Time an idle connection is kept open.                                    |
| `tls-handshake-timeout`   | `10s`   | Time to complete the TLS handshake.                                      |
| `response-header-timeout` | none    | Time to wait for the response headers once the request is written. The route timeout bounds it anyway. |
| `max-idle-conns`          | `200`   | Idle connections kept open, all backends together.                       |
| `max-idle-conns-per-host` | `200`   | Idle connections kept open per backend host.                             |
| `max-conns-per-host`      | `200`   | Connections per backend host, in use and idle.                           |

Settings under `mtls`:

| Key       | Required                 | Description                                                              |
|-----------|--------------------------|--------------------------------------------------------------------------|
| `enabled` | yes                      | `true` to present the client certificate.                                |
| `cert`    | when `enabled` is `true` | Client certificate, PEM encoded.                                         |
| `key`     | when `enabled` is `true` | Private key of the client certificate, PEM encoded.                      |
| `ca`      | when `enabled` is `true` | Certificates the backend certificates are verified against, PEM encoded. |

`cert`, `key` and `ca` hold the PEM text itself, not a file name. In YAML a block scalar (`|`) keeps the
line breaks of the PEM text; in JSON they are written as `\n`. With `mtls` enabled the certificates of the
backends are verified against `ca` instead of the certificates the system trusts.

```yaml
gateway:
  httpclient:
    mtls:
      enabled: true
      cert: |
        -----BEGIN CERTIFICATE-----
        <base64 of the client certificate>
        -----END CERTIFICATE-----
      key: |
        -----BEGIN PRIVATE KEY-----
        <base64 of the private key of the client certificate>
        -----END PRIVATE KEY-----
      ca: |
        -----BEGIN CERTIFICATE-----
        <base64 of the certificate of the authority that signs the backend certificates>
        -----END CERTIFICATE-----
  routes:
    - id: ledger
      uri: https://ledger.internal.example.org
      predicates:
        - name: Path
          args:
            patterns:
              - /ledger/**
```

The lines in angle brackets stand for your own PEM content: the test that builds the examples puts there a
certificate and a key it generates.

* The client has no timeout of its own: every request is bound by the timeout of its route.
* Redirects are not followed: `3xx` responses are forwarded to the client.
* A proxy is taken from the `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` environment variables.

```yaml
gateway:
  httpclient:
    enable-http2: true
    disable-compression: false
    insecure-tls-verify: false
    pool:
      timeout: 5s
      keep-alive: 30s
      idle-conn-timeout: 90s
      tls-handshake-timeout: 5s
      response-header-timeout: 10s
      max-idle-conns: 100
      max-idle-conns-per-host: 20
      max-conns-per-host: 50
  routes:
    - id: catalog
      uri: https://catalog.internal.example.org
      predicates:
        - name: Path
          args:
            patterns:
              - /catalog/**
```

## Error responses

A backend response is forwarded to the client as it is, whatever its status. The gateway answers on its own
in these cases. Apart from `404`, the body carries no message:

| Status                      | When                                                                             |
|-----------------------------|----------------------------------------------------------------------------------|
| `404 Not Found`             | No route matches the request. The body is `route not found`.                     |
| `429 Too Many Requests`     | A `RateLimit` filter rejected the request. It carries `Retry-After` when the limiter tells how long to wait. |
| `499`                       | The client closed the request before it was answered.                            |
| `500 Internal Server Error` | A filter returned an error or panicked, or any other unexpected error.           |
| `502 Bad Gateway`           | The request to the backend failed, for instance because the connection was refused. |
| `503 Service Unavailable`   | The circuit breaker of the route rejected the request.                           |
| `504 Gateway Timeout`       | The route timeout expired.                                                       |

All but `404` come from the error handler, which `WithErrorHandler` replaces. `404` comes from the not found
handler, which `WithNotFoundHandler` replaces. See [Bootstrap options](#bootstrap-options).

## Logging

The gateway logs with `log/slog`, through the logger given to `WithLogger` or `slog.Default()` when there is
none.

* Every log line of a route carries the route id in the `route` attribute. That covers the lines of the
  filters, of the error handler and of the circuit breaker.
* The error handler logs every error it answers: `429` and `499` at `warn` level, the rest at `error` level.
* A circuit breaker logs each state change as `circuit breaker state changed`, with the attributes `breaker`
  (the route id), `from` and `to` (`closed`, `open` or `half-open`). Opening is logged at `warn` level, the
  other changes at `info` level. The line is written by the request that caused the change, without holding
  up the breaker: a slow log handler delays that request, not the other requests of the route. The lines of
  a breaker are written one at a time and in the order of the changes. While one is being written the next
  lines wait for it, up to 16 of them: beyond that the oldest waiting ones are dropped.
* A panic in a filter is logged with its stack and answered with `500`.

## Bootstrap options

`bootstrap.NewOptionsBuilder(cfg)` returns a builder with the defaults. `Build` returns the options that
`bootstrap.Initialize` takes. `Initialize` returns the `*http.Server`, not started, or an error when the
configuration is not valid or the gateway cannot be built from it. The configuration is validated with
`config.Validate` whether it was read from a file or [built in code](#configuration-built-in-code).

| Method                 | Default                             | Description                                                |
|------------------------|-------------------------------------|------------------------------------------------------------|
| `WithServerOptions`    | see below                           | Port, timeouts and header size of the server.              |
| `WithLogger`           | `slog.Default()`                    | Logger of the gateway. The default one is resolved when `Initialize` runs. |
| `WithErrorHandler`     | `gatewayhandler.BaseErrorHandler()` | Answers the requests that fail. See [Error responses](#error-responses). |
| `WithNotFoundHandler`  | `404` with `route not found`        | `http.Handler` that answers the requests that match no route. |
| `WithCustomHandlers`   | none                                | Extra handlers served by the same server, each with a `Method`, a `Path` and a `Handler`. They are registered in an `http.ServeMux` as `Method Path`, next to the gateway, which handles `/`. |
| `WithCustomFilters`    | none                                | Filters added to the built-in ones. See [Extending the Gateway](#extending-the-gateway). |
| `WithCustomPredicates` | none                                | Predicates added to the built-in ones.                     |

Fields of `bootstrap.ServerOpts`. `WithServerOptions` only takes the fields that are not zero, the others
keep their default. A zero or negative timeout means no timeout, but zero cannot be set through
`WithServerOptions` for that reason: pass a negative value to disable a timeout. `ServerOpts` also has a
`CustomHandlers` field, which is the same as calling `WithCustomHandlers`:

| Field               | Default   | Description                                                              |
|---------------------|-----------|--------------------------------------------------------------------------|
| `Port`              | `8000`    | Port the server listens on.                                              |
| `ReadHeaderTimeout` | `2s`      | Time to read the request headers.                                        |
| `ReadTimeout`       | `10s`     | Time to read the whole request. Raised over the longest route timeout.   |
| `WriteTimeout`      | `10s`     | Time to write the whole response. Raised over the longest route timeout. |
| `IdleTimeout`       | `60s`     | Time a keep-alive connection waits for the next request.                 |
| `MaxHeaderBytes`    | `1048576` | Largest size of the request headers.                                     |

```go
opts := bootstrap.NewOptionsBuilder(cfg).
    WithServerOptions(bootstrap.ServerOpts{Port: 9000, ReadTimeout: 30 * time.Second}).
    WithLogger(slog.New(slog.NewJSONHandler(os.Stdout, nil))).
    WithNotFoundHandler(http.NotFoundHandler()).
    WithCustomHandlers(bootstrap.CustomHandler{
        Method: http.MethodGet,
        Path:   "/health",
        Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
            w.WriteHeader(http.StatusNoContent)
        }),
    }).
    Build()
server, err := bootstrap.Initialize(opts)
```

## Extending the Gateway

* **Custom Filters**: Implement `gateway.Filter` and a `gateway.FilterBuilder` that creates it from the `args`
  of the configuration, and pass them to `WithCustomFilters` under the name the configuration uses.
* **Custom Predicates**: Implement `gateway.Predicate` and a `gateway.PredicateBuilder`, and pass them to
  `WithCustomPredicates`.

```go
type stampFilter struct {
    value string
}

func (f *stampFilter) PreProcess(ctx *gateway.Context) error {
    ctx.Request.Headers.Set("X-Stamp", f.value)
    return nil
}

func (f *stampFilter) PostProcess(_ *gateway.Context) error { return nil }

func (f *stampFilter) Name() string { return "Stamp" }

func newOptions(cfg *config.Config) *bootstrap.Options {
    return bootstrap.NewOptionsBuilder(cfg).
        WithCustomFilters(bootstrap.CustomFilter{
            Name: "Stamp",
            Builder: gateway.FilterBuilderFunc(func(args map[string]any) (gateway.Filter, error) {
                value, _ := args["value"].(string)
                return &stampFilter{value: value}, nil
            }),
        }).
        Build()
}
```

* `PreProcess` runs before the backend is called and `PostProcess` once its response headers have arrived.
  An error from either stops the request and goes to the error handler.
* Custom filters and predicates are scoped to the gateway being initialized. They are not added to the
  global `filter.BuilderRegistry` and `predicate.BuilderRegistry`, so two gateways in one process do not see
  each other's. One registered under the name of a built-in filter or predicate replaces it for that gateway
  only.
* Custom rate limiters and rate limit keys are the exception: they are registered in the global
  `ratelimit.RateLimiterBuilderRegistry` and `ratelimit.KeyFuncBuilderRegistry`, with their `Register`
  method, and are visible to every gateway of the process. A limiter that also implements
  `ratelimit.RetryAfterRateLimiter` makes its `429` responses carry `Retry-After`.

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
* [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3): For parsing YAML configuration files.

## Contributing

Contributions are welcome! Please fork the repository and submit a pull request with your enhancements or bug fixes.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

For more detailed information, refer to the [DeepWiki documentation](https://deepwiki.com/Drathveloper/go-cloud-gateway).

---


