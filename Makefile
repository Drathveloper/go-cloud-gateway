GO_BIN=$(shell which go)
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)
LINT_FLAGS ?= --timeout 5m
GOLANGCI_LINT_CONFIG ?= .golangci.yml

ifeq ($(GO_BIN),)
	$(error "go executable not found")
endif

setup:
	@GOPATH=$(shell $(GO_BIN) env GOPATH)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(GOPATH)/bin v2.1.6

lint-check:
ifndef GOLANGCI_LINT
	$(error "golangci-lint is not installed. Run 'make setup' to get all project dependencies")
endif

lint: lint-check
	@echo "Running golangci-lint..."
	$(GOLANGCI_LINT) run $(LINT_FLAGS) --config $(GOLANGCI_LINT_CONFIG)
	@echo "Lint finished"

fmt:
	@echo "Formatting code..."
	$(shell $(GO_BIN) fmt ./...)
	@echo "Formatting finished"
