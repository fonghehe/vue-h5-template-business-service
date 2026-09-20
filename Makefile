SHELL := /bin/bash

BINARY  := server
IMAGE   := vue-h5-template-business-service
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := help

.PHONY: help build run test test-race vet lint fmt check docker migrate migrate-status clean

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Compile the server binary into bin/
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/server

run: ## Run the server locally
	go run ./cmd/server

test: ## Run unit and integration tests
	go test ./...

test-race: ## Run tests with the race detector and write coverage
	go test -race -coverprofile=coverage.out ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (install it first: https://golangci-lint.run)
	golangci-lint run ./...

fmt: ## Format all Go source
	gofmt -w .

check: fmt vet test ## Format, vet and test in one pass

migrate: ## Apply database migrations without starting HTTP or seeding
	go run ./cmd/migrate up

migrate-status: ## Show applied and pending migrations
	go run ./cmd/migrate status

docker: ## Build the production image
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):latest .

clean: ## Remove build artifacts
	rm -rf bin coverage.out
