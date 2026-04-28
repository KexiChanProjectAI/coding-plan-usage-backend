.PHONY: test test-race test-bench lint build clean

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
