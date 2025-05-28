GO_BIN=$(shell which go)

GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)
LINT_FLAGS ?= --timeout 5m
GOLANGCI_LINT_CONFIG ?= .golangci.yml

TEST_PKGS := $(shell go list ./... | grep -v /vendor/)
COVERAGE_DIR := coverage
COVERAGE_FILE := $(COVERAGE_DIR)/coverage.out
COVERAGE_HTML := $(COVERAGE_DIR)/coverage.html

ifeq ($(GO_BIN),)
	$(error "go executable not found")
endif

.PHONY: test test-unit test-cover test-html view-cover clean

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

test: test-cover

test-unit:
	@echo "➜ Running unit tests..."
	go test -v $(TEST_PKGS)

test-cover:
	@echo "➜ Running unit tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_FILE) $(TEST_PKGS)
	@echo "✔ Coverage saved at $(COVERAGE_FILE)"

test-html: test-cover
	@echo "➜ Generating HTML coverage report..."
	go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "✔ Report saved at $(COVERAGE_HTML)"

view-cover: test-html
	@if [ -f $(COVERAGE_HTML) ]; then \
		if command -v xdg-open > /dev/null; then \
			xdg-open $(COVERAGE_HTML); \
		elif command -v open > /dev/null; then \
			open $(COVERAGE_HTML); \
		else \
			echo "Open file://$(abspath $(COVERAGE_HTML))"; \
		fi \
	else \
		echo "Error: test-html required to be run first"; \
		exit 1; \
	fi

clean:
	@rm -rf $(COVERAGE_DIR)
	@echo "✔ Coverage dir cleaned"
