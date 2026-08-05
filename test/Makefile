# Makefile for Egress Router CNI E2E Test

# Binary name
BINARY_NAME := egress-router-cni-tests-ext

# Build directory
BUILD_DIR := bin

# Go module and package
GO_MODULE := github.com/openshift/egress-router-cni

# Go build flags
LDFLAGS :=

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	cd cmd && go build -o ../$(BUILD_DIR)/$(BINARY_NAME) -ldflags="$(LDFLAGS)" .

# Build for Linux (useful for container builds)
.PHONY: build-linux
build-linux:
	@echo "Building $(BINARY_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	cd cmd && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -o ../$(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 -ldflags="$(LDFLAGS)" .

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
