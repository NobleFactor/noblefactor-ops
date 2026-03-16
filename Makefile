# SPDX-License-Identifier: MIT
# Copyright (c) 2025 Noble Factor. All rights reserved.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"

# Rustup toolchain bin directory (avoids MacPorts/Homebrew conflicts)
RUSTUP_BIN := $(shell rustup run stable rustc --print sysroot 2>/dev/null)/bin

# Extensions list - populated by included build.mk files
EXTENSIONS :=

# Include all extension build rules
-include extensions/*/build.mk

.PHONY: all build test install deps check-codegen
.PHONY: build-extensions package-extensions clean-extensions
.PHONY: check-wasm-target clean

# =============================================================================
# Main targets
# =============================================================================

all: build-extensions build

build:
	go build $(LDFLAGS) -o bin/star ./cmd/star

test:
	go test ./...

clean: clean-extensions
	rm -rf bin/

# Install binary to GOBIN or ~/.local/bin
install: build
	@mkdir -p $(or $(GOBIN),$(HOME)/.local/bin)
	cp bin/star $(or $(GOBIN),$(HOME)/.local/bin)/
	@echo "Installed to $(or $(GOBIN),$(HOME)/.local/bin)"

# Verify all dependencies are available
deps:
	go mod download
	go mod verify

# Verify generated provider files match LKG codegen output
CODEGEN_BRANCH ?= develop
check-codegen:
	scripts/check-codegen.sh --branch=$(CODEGEN_BRANCH)

# =============================================================================
# Extension targets
# =============================================================================

# Build all extensions (calls each extension's -build target)
build-extensions: check-wasm-target $(addsuffix -build,$(EXTENSIONS))

# Package all extensions for distribution
package-extensions: $(addsuffix -package,$(EXTENSIONS))

# Clean all extension build artifacts
clean-extensions: $(addsuffix -clean,$(EXTENSIONS))

# Check that wasm32-wasip1 target is installed
check-wasm-target:
	@rustup target list --installed 2>/dev/null | grep -q wasm32-wasip1 || \
		(echo "Error: wasm32-wasip1 target not installed. Run: rustup target add wasm32-wasip1" && exit 1)

# =============================================================================
# Help
# =============================================================================

.PHONY: help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Main targets:"
	@echo "  all                Build extensions and Go binary (default)"
	@echo "  build              Build Go binary only"
	@echo "  test               Run Go tests"
	@echo "  clean              Clean all build artifacts"
	@echo "  install            Install binary to GOBIN or ~/.local/bin"
	@echo "  check-codegen      Verify gen/ files match LKG codegen (CODEGEN_BRANCH=develop)"
	@echo ""
	@echo "Extension targets:"
	@echo "  build-extensions   Build all WASM extensions"
	@echo "  package-extensions Package all extensions for distribution"
	@echo "  clean-extensions   Clean extension build artifacts"
	@echo ""
	@echo "Registered extensions:"
	@for ext in $(EXTENSIONS); do echo "  $$ext"; done
