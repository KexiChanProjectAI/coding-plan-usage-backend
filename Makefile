.PHONY: test test-race test-bench lint build clean tidy verify frontend-install frontend-typecheck frontend-test frontend-build verify-all

# Binary name
BINARY_NAME=ucpqa
MAIN_PATH=./cmd/api

# Go parameters
GOCMD=go
GOTEST=$(GOCMD) test
GOBUILD=$(GOCMD) build
GOLINT=golangci-lint run

# Test flags
TEST_FLAGS=-v -count=1
RACE_FLAGS=-race

# Default target
all: test lint build

# Run unit tests
test:
	$(GOTEST) $(TEST_FLAGS) ./...

# Run unit tests with race detector
test-race:
	$(GOTEST) $(RACE_FLAGS) $(TEST_FLAGS) ./...

# Run benchmarks
test-bench:
	$(GOTEST) -bench=. -benchmem -count=3 ./...

# Run linter
lint:
	$(GOLINT) ./...

# Build the binary
build:
	$(GOBUILD) -o $(BINARY_NAME) $(MAIN_PATH)

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	go clean

# Tidy dependencies
tidy:
	go mod tidy

# Verify dependencies
verify:
	go mod verify

# Install frontend dependencies
frontend-install:
	cd frontend && npm ci

# Run frontend typecheck
frontend-typecheck:
	cd frontend && npm run typecheck

# Run frontend tests
frontend-test:
	cd frontend && npm test -- --run

# Build frontend assets
frontend-build:
	cd frontend && npm run build

# Run full backend + frontend verification
verify-all: test lint build frontend-install frontend-typecheck frontend-test frontend-build
