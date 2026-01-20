.PHONY: all build test clean install-deps fmt vet lint test-coverage build-metron build-aqara-test build-bot build-win-agent build-mac-agent release-win-agent run-aqara-test help

# Variables
BINARY_NAME=metron
AQARA_TEST_BINARY=aqara-test
BOT_BINARY=metron-bot
WIN_AGENT_BINARY=metron-win-agent.exe
MAC_AGENT_BINARY=metron-agent
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
	@echo "  make build-win-agent    - Build Windows agent (cross-compile)"
	@echo "  make build-mac-agent    - Build macOS agent (debug, logging-only)"
	@echo "  make release-win-agent  - Build Windows agent release package (zip)"
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
build: build-metron build-aqara-test build-bot
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

## build-bot: Build Telegram bot
build-bot:
	@echo "Building $(BOT_BINARY)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BOT_BINARY) ./cmd/metron-bot
	@echo "Built: $(BUILD_DIR)/$(BOT_BINARY)"

## build-win-agent: Build Windows agent (cross-compile from macOS/Linux)
build-win-agent:
	@echo "Building $(WIN_AGENT_BINARY) for Windows amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "-H windowsgui" -o $(BUILD_DIR)/$(WIN_AGENT_BINARY) ./cmd/metron-win-agent
	@echo "Built: $(BUILD_DIR)/$(WIN_AGENT_BINARY)"

## build-mac-agent: Build macOS agent (debug, logging-only enforcement)
build-mac-agent:
	@echo "Building $(MAC_AGENT_BINARY) for macOS (debug)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(MAC_AGENT_BINARY) ./cmd/metron-win-agent
	@echo "Built: $(BUILD_DIR)/$(MAC_AGENT_BINARY)"

## release-win-agent: Build Windows agent release package (zip)
release-win-agent: build-win-agent
	@echo "Creating Windows agent release package..."
	@mkdir -p $(BUILD_DIR)/metron-win-agent
	@cp $(BUILD_DIR)/$(WIN_AGENT_BINARY) $(BUILD_DIR)/metron-win-agent/
	@cp deploy/win-agent/_setup.ps1 $(BUILD_DIR)/metron-win-agent/
	@cp deploy/win-agent/INSTALL.bat $(BUILD_DIR)/metron-win-agent/
	@cp deploy/win-agent/README.txt $(BUILD_DIR)/metron-win-agent/
	@# Generate config.txt from config.json and bot-config.json if they exist
	@if [ -f config.json ] && [ -f bot-config.json ]; then \
		DEVICE_ID=$$(jq -r '.devices[] | select(.driver == "passive") | .id' config.json | head -1); \
		TOKEN=$$(jq -r '.devices[] | select(.driver == "passive") | .parameters.agent_token' config.json | head -1); \
		URL=$$(jq -r '.metron.base_url' bot-config.json); \
		echo "# Metron Windows Agent Configuration" > $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "# Generated from config.json and bot-config.json" >> $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "" >> $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "# Required settings" >> $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "DEVICE_ID=$$DEVICE_ID" >> $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "TOKEN=$$TOKEN" >> $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "URL=$$URL" >> $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "" >> $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "# Optional settings (uncomment to change defaults)" >> $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "# POLL_INTERVAL=15" >> $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "# GRACE_PERIOD=30" >> $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "# LOG_LEVEL=info" >> $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "# LOG_FORMAT=json" >> $(BUILD_DIR)/metron-win-agent/config.txt; \
		echo "Config generated from production config files"; \
	else \
		cp deploy/win-agent/config.txt $(BUILD_DIR)/metron-win-agent/; \
		echo "Using template config (no config.json/bot-config.json found)"; \
	fi
	@cd $(BUILD_DIR) && zip -r metron-win-agent.zip metron-win-agent/
	@rm -rf $(BUILD_DIR)/metron-win-agent
	@echo "Built: $(BUILD_DIR)/metron-win-agent.zip"

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
