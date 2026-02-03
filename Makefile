.PHONY: build test clean install run-init run-push run-pull run-both help

# Build variables
BINARY_NAME=sync
BUILD_DIR=.
CMD_DIR=./cmd/sync

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build with optimizations for production
build-prod:
	@echo "Building $(BINARY_NAME) for production..."
	@go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Production build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run all tests
test:
	@echo "Running tests..."
	@go test ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -cover ./...

# Generate detailed coverage report
coverage:
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies installed"

# Install to GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME) to $$GOPATH/bin..."
	@cp $(BINARY_NAME) $$GOPATH/bin/
	@echo "Install complete"

# Run init command
run-init:
	@./$(BINARY_NAME) init

# Run push command
run-push:
	@./$(BINARY_NAME) push

# Run push with dry-run
run-push-dry:
	@./$(BINARY_NAME) push --dry-run

# Run pull command
run-pull:
	@./$(BINARY_NAME) pull

# Run pull with dry-run
run-pull-dry:
	@./$(BINARY_NAME) pull --dry-run

# Run both command
run-both:
	@./$(BINARY_NAME) both

# Run both with dry-run
run-both-dry:
	@./$(BINARY_NAME) both --dry-run

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	@golangci-lint run

# Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  build-prod     - Build with production optimizations"
	@echo "  test           - Run all tests"
	@echo "  test-verbose   - Run tests with verbose output"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  coverage       - Generate HTML coverage report"
	@echo "  clean          - Remove build artifacts"
	@echo "  deps           - Install dependencies"
	@echo "  install        - Install binary to GOPATH/bin"
	@echo "  run-init       - Run init command"
	@echo "  run-push       - Run push command"
	@echo "  run-push-dry   - Run push with --dry-run"
	@echo "  run-pull       - Run pull command"
	@echo "  run-pull-dry   - Run pull with --dry-run"
	@echo "  run-both       - Run both command"
	@echo "  run-both-dry   - Run both with --dry-run"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code (requires golangci-lint)"
	@echo "  help           - Show this help message"

.DEFAULT_GOAL := help
