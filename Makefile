.PHONY: all build test clean install-deps fmt vet lint test-coverage build-aqara-test run-aqara-test help

# Variables
BINARY_NAME=metron
AQARA_TEST_BINARY=aqara-test
BUILD_DIR=bin
COVERAGE_FILE=coverage.out
COVERAGE_HTML=coverage.html

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Default target
all: clean test build

## help: Display this help message
help:
	@echo "Available targets:"
	@echo "  make build              - Build all binaries"
	@echo "  make build-aqara-test   - Build Aqara test CLI"
	@echo "  make test               - Run all tests"
	@echo "  make test-coverage      - Run tests with coverage report"
	@echo "  make clean              - Remove build artifacts"
	@echo "  make fmt                - Format code"
	@echo "  make vet                - Run go vet"
	@echo "  make lint               - Run golangci-lint (if installed)"
	@echo "  make install-deps       - Download dependencies"
	@echo "  make run-aqara-test     - Run Aqara test (default: pin action)"
	@echo "  make help               - Show this help message"

## build: Build all binaries
build: build-metron build-aqara-test
	@echo "All binaries built successfully"

## build-metron: Build main application
build-metron:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/metron
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

## build-aqara-test: Build Aqara test CLI
build-aqara-test:
	@echo "Building $(AQARA_TEST_BINARY)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(AQARA_TEST_BINARY) ./cmd/aqara-test
	@echo "Built: $(BUILD_DIR)/$(AQARA_TEST_BINARY)"

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=$(COVERAGE_FILE) ./...
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"
	@echo "Open $(COVERAGE_HTML) in your browser to view the coverage report"

## test-race: Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	$(GOTEST) -race -v ./...

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	rm -f *.db *.db-shm *.db-wal
	@echo "Clean complete"

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...
	@echo "Format complete"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "Vet complete"

## lint: Run golangci-lint (if installed)
lint:
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install from: https://golangci-lint.run/usage/install/"; \
	fi

## install-deps: Download and tidy dependencies
install-deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies installed"

## run-metron: Run main application
run-metron: build-metron
	@echo "Starting Metron application..."
	./$(BUILD_DIR)/$(BINARY_NAME)

## run-aqara-test: Run Aqara test CLI (use ACTION=warn or ACTION=off to change action)
run-aqara-test: build-aqara-test
	@echo "Running Aqara test..."
	./$(BUILD_DIR)/$(AQARA_TEST_BINARY) -action $(or $(ACTION),pin)

## Quick test targets for different actions
test-pin: build-aqara-test
	@echo "Testing PIN entry scene..."
	./$(BUILD_DIR)/$(AQARA_TEST_BINARY) -action pin

test-warn: build-aqara-test
	@echo "Testing warning scene..."
	./$(BUILD_DIR)/$(AQARA_TEST_BINARY) -action warn

test-off: build-aqara-test
	@echo "Testing power-off scene..."
	./$(BUILD_DIR)/$(AQARA_TEST_BINARY) -action off
