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

.PHONY: help
help:
	@echo "Springfield CSI Driver - Makefile targets:"
	@echo "  build       - Build the CSI driver binary"
	@echo "  test        - Run tests"
	@echo "  fmt         - Format code"
	@echo "  vet         - Run go vet"
	@echo "  lint        - Run all linters"
	@echo "  mod         - Download and tidy dependencies"
	@echo "  image       - Build container image"
	@echo "  push        - Push container image"
	@echo "  clean       - Clean build artifacts"
	@echo ""
	@echo "Variables:"
	@echo "  REGISTRY    - Container registry (default: $(REGISTRY))"
	@echo "  IMAGE_NAME  - Image name (default: $(IMAGE_NAME))"
	@echo "  IMAGE_TAG   - Image tag (default: $(IMAGE_TAG))"
	@echo "  GOOS        - Target OS (default: $(GOOS))"
	@echo "  GOARCH      - Target architecture (default: $(GOARCH))"
