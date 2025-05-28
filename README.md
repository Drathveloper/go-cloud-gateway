[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/Drathveloper/go-cloud-gateway)

# Overview

go-cloud-gateway aims to be a configurable API gateway written in Go and heavily inspired in spring cloud gateway

The go-cloud-gateway is designed to act as a reverse proxy and API gateway that routes incoming HTTP requests to backend services based on configurable predicates and applies request/response transformations through a filter pipeline.

The go-cloud-gateway serves as a lightweight, configuration-driven API gateway that provides:
- Route-based request forwarding using configurable predicates for path, method, and host matching 
- Request/response transformation through a pluggable filter system 
- Configuration management supporting both YAML and JSON formats with validation 
- Extensible architecture using factory patterns for filters and predicates

