# codebak Makefile

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
BINARY := codebak
BUILD_DIR := dist

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod

# Build targets
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64

.PHONY: all build build-all install test lint clean version help tidy

all: build

## build: Build for current platform
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY) ./cmd/codebak
	@echo "Built $(BINARY) version $(VERSION)"

## build-all: Cross-compile for all platforms
build-all: clean
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d'/' -f1) \
		GOARCH=$$(echo $$platform | cut -d'/' -f2) \
		OUTPUT=$(BUILD_DIR)/$(BINARY)-$$(echo $$platform | tr '/' '-'); \
		echo "Building $$OUTPUT..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH $(GOBUILD) $(LDFLAGS) -o $$OUTPUT ./cmd/codebak; \
	done
	@echo "Built binaries in $(BUILD_DIR)/"
	@ls -la $(BUILD_DIR)/

## install: Install to ~/go/bin
install: build
	@mkdir -p ~/go/bin
	cp $(BINARY) ~/go/bin/$(BINARY)
	@echo "Installed $(BINARY) to ~/go/bin/"

## test: Run all tests
test:
	$(GOTEST) -v ./...

## test-coverage: Run tests with coverage report
test-coverage:
	$(GOTEST) -cover ./...

## lint: Run golangci-lint (requires golangci-lint installed)
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: brew install golangci-lint" && exit 1)
	golangci-lint run ./...

## tidy: Tidy and verify go modules
tidy:
	$(GOMOD) tidy
	$(GOMOD) verify

## clean: Remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)
	rm -f codebak-test

## version: Show version
version:
	@echo "Version: $(VERSION)"

## help: Show this help
help:
	@echo "codebak Makefile targets:"
	@echo ""
	@grep -E '^##' Makefile | sed 's/## /  /'
	@echo ""
	@echo "Current version: $(VERSION)"
