# Pulsar Makefile
# Provides standard targets for common development tasks

# Version information. BUILD_TIME and GIT_COMMIT are deferred so that targets
# which never reference LDFLAGS (help, fmt, vet, test, ...) skip the shell calls.
VERSION ?= dev
BUILD_TIME = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags for version injection (vars are no-ops until declared in main)
LDFLAGS = -X main.Version=$(VERSION) \
          -X main.BuildTime=$(BUILD_TIME) \
          -X main.GitCommit=$(GIT_COMMIT)

# Build the Pulsar application
build:
	go build -ldflags "$(LDFLAGS)" -o pulsar .

# Build the Pulsar application with version information
build-release:
	@if [ "$(VERSION)" = "dev" ]; then \
		echo "Error: VERSION must be set for release builds. Usage: make build-release VERSION=1.2.3"; \
		exit 1; \
	fi
	go build -ldflags "$(LDFLAGS)" -o pulsar .

# Run all tests
test:
	go test ./...

# Run tests with verbose output and coverage
test-verbose:
	go test -v -cover ./...

# Run only integration tests
test-integration:
	INTEGRATION=1 go test ./...

# Run integration tests with verbose output
test-integration-verbose:
	INTEGRATION=1 go test -v ./...

# Run both unit and integration tests
test-all: test test-integration

# Generate test coverage report
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

# Run benchmark tests
benchmarks:
	go test -bench=. ./...

# Run benchmarks with memory profiling
benchmarks-mem:
	go test -bench=. -benchmem ./...

# Format Go code
fmt:
	go fmt ./...

# Run go vet for static analysis
vet:
	go vet ./...

# Run linter. .golangci.yml declares version: "2" — golangci-lint itself
# fails loudly if a v1 binary is on PATH or if the binary is missing.
lint:
	golangci-lint run

# Run full validation suite
check: fmt vet lint test

# Clean build artifacts
clean:
	rm -f pulsar
	rm -rf dist/
	rm -f coverage.out coverage.html

# Install the application
install:
	go install -ldflags "$(LDFLAGS)" .

# Clean up go.mod and go.sum
deps-tidy:
	go mod tidy

# Update dependencies to latest versions
deps-update:
	go get -u ./...
	go mod tidy

# Run security scan. Requires gosec on PATH:
# go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
security-scan:
	gosec ./...

# List all Go functions in the project
go-functions:
	@echo "Finding all functions in the project..."
	@grep -r "^func " . --include="*.go" | grep -v vendor/

# Show help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  build                 - Build the Pulsar application with version info"
	@echo "  build-release         - Build release version (requires VERSION=x.y.z)"
	@echo "  install               - Install the application"
	@echo "  clean                 - Clean build artifacts and coverage files"
	@echo ""
	@echo "Testing targets:"
	@echo "  test                  - Run Go unit tests"
	@echo "  test-verbose          - Run tests with verbose output and coverage"
	@echo "  test-integration      - Run only integration tests"
	@echo "  test-integration-verbose - Run integration tests with verbose output"
	@echo "  test-all              - Run both unit and integration tests"
	@echo "  test-coverage         - Generate test coverage report (HTML)"
	@echo ""
	@echo "Benchmark targets:"
	@echo "  benchmarks            - Run benchmark tests"
	@echo "  benchmarks-mem        - Run benchmarks with memory profiling"
	@echo ""
	@echo "Code quality targets:"
	@echo "  fmt                   - Format Go code"
	@echo "  vet                   - Run go vet for static analysis"
	@echo "  lint                  - Run linter (requires golangci-lint v2+)"
	@echo "  check                 - Run full validation suite (fmt, vet, lint, test)"
	@echo "  security-scan         - Run security analysis (requires gosec)"
	@echo ""
	@echo "Dependency management:"
	@echo "  deps-tidy             - Clean up go.mod and go.sum"
	@echo "  deps-update           - Update dependencies to latest versions"
	@echo ""
	@echo "Development utilities:"
	@echo "  go-functions          - List all Go functions in the project"
	@echo "  help                  - Show this help message"
	@echo ""
	@echo "Build examples:"
	@echo "  make build                       - Build with dev version"
	@echo "  make build VERSION=1.2.3         - Build with specific version"
	@echo "  make build-release VERSION=1.2.3 - Build release version"

.PHONY: build build-release test test-verbose test-integration test-integration-verbose test-all test-coverage benchmarks benchmarks-mem fmt vet lint check clean install deps-tidy deps-update security-scan go-functions help
