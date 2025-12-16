.PHONY: build test lint clean install run help integration-test integration-test-coverage coverage-all

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X github.com/tomatool/tomato/version.Version=$(VERSION) \
                     -X github.com/tomatool/tomato/version.Commit=$(COMMIT) \
                     -X github.com/tomatool/tomato/version.BuildDate=$(BUILD_DATE)"

# Go variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
BINARY_NAME := tomato
BINARY_PATH := ./bin/$(BINARY_NAME)

## help: Show this help message
help:
	@echo "Tomato v2 - Behavioral Testing Toolkit"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build the tomato binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p ./bin
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH) .

## install: Install tomato to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY_NAME) .

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linters
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## tidy: Tidy go modules
tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf ./bin
	@rm -rf ./coverage ./coverage-merged
	@rm -f coverage.out coverage.html coverage-integration.out coverage-integration.html coverage-all.out coverage-all.html

## run: Run tomato with example config
run: build
	$(BINARY_PATH) run

## dev: Build and run in development mode
dev: build
	$(BINARY_PATH) --help

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t tomatool/tomato:$(VERSION) .
	docker tag tomatool/tomato:$(VERSION) tomatool/tomato:latest

## integration-test: Run integration tests using tomato to test itself
integration-test: build
	@echo "Running integration tests..."
	$(BINARY_PATH) run -c ./tests/tomato.yml

## integration-test-coverage: Run integration tests with coverage (Cloudflare technique)
integration-test-coverage:
	@echo "Building coverage-instrumented binary..."
	@mkdir -p ./bin
	$(GOCMD) build -cover -covermode=atomic -coverpkg=./... $(LDFLAGS) -o ./bin/tomato-coverage .
	@echo "Running integration tests with coverage..."
	@mkdir -p ./coverage
	GOCOVERDIR=./coverage ./bin/tomato-coverage run -c ./tests/tomato.yml || true
	@echo "Converting coverage data..."
	$(GOCMD) tool covdata textfmt -i=./coverage -o=coverage-integration.out
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage-integration.out -o coverage-integration.html
	@echo "Integration coverage report: coverage-integration.html"

## coverage-all: Run both unit tests and integration tests with combined coverage
coverage-all: test-coverage integration-test-coverage
	@echo "Merging coverage reports..."
	@if [ -f coverage.out ] && [ -f coverage-integration.out ]; then \
		$(GOCMD) tool covdata merge -i=./coverage -o=./coverage-merged 2>/dev/null || true; \
		cat coverage.out > coverage-all.out; \
		tail -n +2 coverage-integration.out >> coverage-all.out; \
		$(GOCMD) tool cover -html=coverage-all.out -o coverage-all.html; \
		echo "Combined coverage report: coverage-all.html"; \
	fi

## release: Create a release build for multiple platforms
release: clean
	@echo "Building releases..."
	@mkdir -p ./bin/release
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o ./bin/release/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o ./bin/release/$(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o ./bin/release/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o ./bin/release/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o ./bin/release/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Releases built in ./bin/release/"
