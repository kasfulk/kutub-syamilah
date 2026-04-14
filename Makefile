.PHONY: build test lint clean run fmt vet bench

# Variables
BINARY_NAME=kutub-syamilah
BUILD_DIR=bin
GO=go
GOFLAGS=-v

# Build the application
build:
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/api

# Run the application
run:
	$(GO) run ./cmd/api

# Run tests with race detector and coverage
test:
	$(GO) test -v -race -coverprofile=coverage.out ./...

# Run tests with coverage report
test-coverage: test
	$(GO) tool cover -html=coverage.out

# Format code
fmt:
	$(GO) fmt ./...
	goimports -w .

# Vet code
vet:
	$(GO) vet ./...

# Run linters
lint: vet
	golangci-lint run ./...

# Run benchmarks
bench:
	$(GO) test -bench=. -benchmem -count=6 ./internal/repository/... | tee /tmp/report-1.txt

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

# Install dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

# Build for Linux (production)
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -a -installsuffix cgo -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/api

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application"
	@echo "  test          - Run tests with race detector"
	@echo "  test-coverage - Run tests with HTML coverage report"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  lint          - Run golangci-lint"
	@echo "  bench         - Run benchmarks"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  build-linux   - Cross-compile for Linux"
