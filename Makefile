# Makefile for LangGraph-Go

.PHONY: all build examples test test-verbose test-integration test-cover clean install fmt vet lint help

# Variables
BUILD_DIR := ./build
EXAMPLES_DIR := ./examples
GO := go
GOFLAGS := -v

# All example directories
EXAMPLES := $(shell find $(EXAMPLES_DIR) -mindepth 1 -maxdepth 1 -type d)
EXAMPLE_NAMES := $(notdir $(EXAMPLES))
EXAMPLE_BINS := $(addprefix $(BUILD_DIR)/, $(EXAMPLE_NAMES))

# Default target
all: build examples

# Help target
help:
	@echo "LangGraph-Go Makefile targets:"
	@echo "  make build           - Build the main library"
	@echo "  make examples        - Build all examples to ./build"
	@echo "  make test            - Run all tests"
	@echo "  make test-verbose    - Run tests with verbose output"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-cover      - Run tests with coverage report"
	@echo "  make bench           - Run benchmarks"
	@echo "  make clean           - Remove build artifacts"
	@echo "  make install         - Install dependencies"
	@echo "  make fmt             - Format code"
	@echo "  make vet             - Run go vet"
	@echo "  make lint            - Run golangci-lint (if available)"
	@echo "  make all             - Build library and examples"

# Build the library
build:
	@echo "Building library..."
	@$(GO) build $(GOFLAGS) ./...

# Build all examples
examples: $(EXAMPLE_BINS)

# Pattern rule to build individual examples
$(BUILD_DIR)/%: | $(BUILD_DIR)
	@echo "Building example: $*"
	@$(GO) build $(GOFLAGS) -o $@ $(EXAMPLES_DIR)/$*/main.go

# Create build directory if it doesn't exist
$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

# Run tests
test:
	@echo "Running tests..."
	@$(GO) test ./... -short -timeout 2m

# Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	@$(GO) test -v ./... -short

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	@$(GO) test ./... -run Integration

# Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	@$(GO) test -cover ./...
	@$(GO) test -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@$(GO) test -bench=. -benchmem ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@$(GO) clean

# Install dependencies
install:
	@echo "Installing dependencies..."
	@$(GO) mod download
	@$(GO) mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	@$(GO) fmt ./...

# Run go vet
vet:
	@echo "Running go vet..."
	@$(GO) vet ./...

# Run golangci-lint if available
lint:
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

# Run security checks with gosec if available
security:
	@echo "Running security checks..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not installed, skipping..."; \
	fi

# Build individual examples (can be called as: make example-chatbot)
example-%: $(BUILD_DIR)/%
	@echo "Built example: $*"

# List all examples
list-examples:
	@echo "Available examples:"
	@for example in $(EXAMPLE_NAMES); do \
		echo "  - $$example"; \
	done
