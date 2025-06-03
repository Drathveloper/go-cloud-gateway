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

## Dependencies

Key external libraries are used:

* [`github.com/go-playground/validator/v10`](https://pkg.go.dev/github.com/go-playground/validator/v10): For configuration validation.
* [`github.com/google/uuid`](https://pkg.go.dev/github.com/google/uuid): For generating UUIDs, useful in request tracking.
* [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3): For parsing YAML configuration files.

## Contributing

Contributions are welcome! Please fork the repository and submit a pull request with your enhancements or bug fixes.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

For more detailed information, refer to the [DeepWiki documentation](https://deepwiki.com/Drathveloper/go-cloud-gateway).

---


