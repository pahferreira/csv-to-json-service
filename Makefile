.DEFAULT_GOAL := help

# Build metadata
BIN_DIR   := bin
BINARY    := server
PKG       := ./cmd/server
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -X main.version=$(VERSION)

# Local developer convenience. The service itself only ever reads environment
# variables (ADR-0011); .env just populates them for `make run`. The leading
# dash makes a missing file a no-op, and only the four variables config reads
# are exported, so a stray line in .env cannot clobber PATH for a recipe.
-include .env
export PORT MAX_BYTES MAX_ROWS LOG_LEVEL

# Overridable from the environment or the command line
IMAGE     ?= csv-to-json-service
FUZZ      ?= Fuzz
FUZZ_PKG  ?= ./internal/platform/config
FUZZ_TIME ?= 30s

.PHONY: help build run test lint docker fuzz clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}'

build: ## Compile the server binary into bin/
	go build -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) $(PKG)

run: ## Run the server from source
	go run $(PKG)

test: ## Run all tests
	go test ./...

lint: ## Run golangci-lint over every package
	golangci-lint run ./...

docker: ## Build the container image
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

fuzz: ## Fuzz one target (override FUZZ, FUZZ_PKG, FUZZ_TIME)
	go test -run '^$$' -fuzz '$(FUZZ)' -fuzztime $(FUZZ_TIME) $(FUZZ_PKG)

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
