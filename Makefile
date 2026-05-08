# Copyright (C) 2023-2026  Red Hat, Inc.
# Makefile for Springfield CSI Driver

REGISTRY ?= ghcr.io/springfield-csi
IMAGE_NAME ?= driver
IMAGE_TAG ?= devel
FULL_IMAGE ?= $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GOOS ?= linux
GOARCH ?= amd64

.PHONY: all
all: build

.PHONY: build
build:
	@echo "Building CSI driver binary..."
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-a -ldflags '-extldflags "-static"' \
		-o bin/csi-driver ./cmd/csi-driver

.PHONY: test
test:
	@echo "Running tests..."
	go test -v -race -cover ./pkg/...

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

.PHONY: vet
vet:
	@echo "Running go vet..."
	go vet ./...

.PHONY: lint
lint: fmt vet
	@echo "Running linters..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

.PHONY: mod
mod:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

.PHONY: image
image:
	@echo "Building container image: $(FULL_IMAGE)"
	docker build \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(FULL_IMAGE) \
		-f Dockerfile .

.PHONY: push
push:
	@echo "Pushing container image: $(FULL_IMAGE)"
	docker push $(FULL_IMAGE)

.PHONY: clean
clean:
	@echo "Cleaning up..."
	rm -rf bin/
	go clean -cache

.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	@if [ "$$(id -u)" != "0" ]; then \
		echo "Error: Integration tests require root privileges"; \
		echo "Run: sudo make test-integration"; \
		exit 1; \
	fi
	go test -v -tags=integration -timeout=10m ./test/integration/...

.PHONY: test-sanity
test-sanity:
	@echo "Running CSI sanity tests..."
	@if [ ! -f bin/csi-driver ]; then \
		echo "Error: Driver binary not found. Run 'make build' first"; \
		exit 1; \
	fi
	@if ! command -v csi-sanity >/dev/null 2>&1; then \
		echo "Installing csi-sanity..."; \
		go install github.com/kubernetes-csi/csi-test/v5/cmd/csi-sanity@latest; \
	fi
	./test/sanity/run_sanity.sh

.PHONY: test-e2e
test-e2e:
	@echo "Running E2E tests..."
	go test -v -tags=e2e -timeout=30m ./test/e2e/...

.PHONY: test-all
test-all: test test-integration test-sanity test-e2e
	@echo "All tests passed!"

.PHONY: coverage
coverage:
	@echo "Generating coverage report..."
	go test -coverprofile=coverage.out ./pkg/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
	@go tool cover -func=coverage.out | grep total | awk '{print "Total coverage: " $$3}'

.PHONY: help
help:
	@echo "Springfield CSI Driver - Makefile targets:"
	@echo ""
	@echo "Build:"
	@echo "  build       - Build the CSI driver binary"
	@echo "  image       - Build container image"
	@echo "  push        - Push container image"
	@echo ""
	@echo "Testing:"
	@echo "  test        - Run unit tests"
	@echo "  test-integration - Run integration tests (requires root)"
	@echo "  test-sanity - Run CSI sanity tests"
	@echo "  test-e2e    - Run end-to-end tests"
	@echo "  test-all    - Run all tests"
	@echo "  coverage    - Generate coverage report"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt         - Format code"
	@echo "  vet         - Run go vet"
	@echo "  lint        - Run all linters"
	@echo ""
	@echo "Dependencies:"
	@echo "  mod         - Download and tidy dependencies"
	@echo "  clean       - Clean build artifacts"
	@echo ""
	@echo "Variables:"
	@echo "  REGISTRY    - Container registry (default: $(REGISTRY))"
	@echo "  IMAGE_NAME  - Image name (default: $(IMAGE_NAME))"
	@echo "  IMAGE_TAG   - Image tag (default: $(IMAGE_TAG))"
	@echo "  GOOS        - Target OS (default: $(GOOS))"
	@echo "  GOARCH      - Target architecture (default: $(GOARCH))"
