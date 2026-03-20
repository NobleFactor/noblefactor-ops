# SPDX-License-Identifier: MIT
# Copyright (c) 2025 Noble Factor. All rights reserved.

SHELL := bash
.SHELLFLAGS := -o errexit -o nounset -o pipefail -c
.ONESHELL:
.SILENT:

## PARAMETERS

### VERSION

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"

### CODEGEN

# devlore-cli repo (required for codegen — star runs from this context).
DEVLORE_CLI ?= ../devlore-cli

# Codegen baseline branch for check-codegen.
CODEGEN_BRANCH ?= develop

### WASM

# Rustup toolchain bin directory (avoids MacPorts/Homebrew conflicts).
RUSTUP_BIN := $(shell rustup run stable rustc --print sysroot 2>/dev/null)/bin

## VARIABLES (static)

# Provider source root.
P := internal/provider

# Extensions list — populated by included build.mk files.
EXTENSIONS :=

# Include all extension build rules.
-include extensions/*/build.mk

## TARGETS

.PHONY: all build clean test install deps check-codegen star generate help
.PHONY: build-extensions package-extensions clean-extensions check-wasm-target

##@ Help

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Main targets:"
	@echo "  all                Build extensions and star binary (default)"
	@echo "  build              Build star binary (regenerates providers if needed)"
	@echo "  test               Run Go tests"
	@echo "  clean              Clean all build artifacts and generated code"
	@echo "  install            Install binary to GOBIN or ~/.local/bin"
	@echo "  check-codegen      Verify gen/ files match LKG codegen (CODEGEN_BRANCH=develop)"
	@echo ""
	@echo "Code generation:"
	@echo "  star               Build the star binary"
	@echo "  generate           Regenerate all provider gen/ files"
	@echo ""
	@echo "Extension targets:"
	@echo "  build-extensions   Build all WASM extensions"
	@echo "  package-extensions Package all extensions for distribution"
	@echo "  clean-extensions   Clean extension build artifacts"
	@echo ""
	@echo "Registered extensions:"
	@for ext in $(EXTENSIONS); do echo "  $$ext"; done

##@ Build

all: build-extensions build

star: ## Build the star binary
	go build $(LDFLAGS) -o build/star ./cmd/star

build: generate star ## Build star binary (regenerates providers first)

clean: clean-extensions ## Clean build artifacts
	rm -rf build/

install: build ## Install binary to GOBIN or ~/.local/bin
	@mkdir -p $(or $(GOBIN),$(HOME)/.local/bin)
	cp build/star $(or $(GOBIN),$(HOME)/.local/bin)/
	@echo "Installed to $(or $(GOBIN),$(HOME)/.local/bin)"

deps: ## Verify all dependencies are available
	go mod download
	go mod verify

##@ Test

test: ## Run Go tests
	go test ./...

##@ Quality

check-codegen: ## Verify gen/ files match LKG codegen output
	scripts/check-codegen.sh --branch=$(CODEGEN_BRANCH)

##@ Code Generation

# Each grouped target (&:) fires one star invocation that produces all gen files.
# Generation runs only when provider.go is newer than the gen outputs.
# All providers are access=immediate: receiver + receiver_gen_test + params.

$(P)/commands/gen/params.gen.go \
$(P)/commands/gen/receiver.gen.go \
$(P)/commands/gen/receiver_gen_test.go &: $(P)/commands/provider.go | star
	cd $(DEVLORE_CLI) && $(CURDIR)/build/star devlore actions generate \
		--source=$(CURDIR)/$(P)/commands --gen=true --write=true --output=$(CURDIR)/$(P)/commands

$(P)/config/gen/params.gen.go \
$(P)/config/gen/receiver.gen.go \
$(P)/config/gen/receiver_gen_test.go &: $(P)/config/provider.go | star
	cd $(DEVLORE_CLI) && $(CURDIR)/build/star devlore actions generate \
		--source=$(CURDIR)/$(P)/config --gen=true --write=true --output=$(CURDIR)/$(P)/config

$(P)/goast/gen/params.gen.go \
$(P)/goast/gen/receiver.gen.go \
$(P)/goast/gen/receiver_gen_test.go &: $(P)/goast/provider.go | star
	cd $(DEVLORE_CLI) && $(CURDIR)/build/star devlore actions generate \
		--source=$(CURDIR)/$(P)/goast --gen=true --write=true --output=$(CURDIR)/$(P)/goast

$(P)/lint/gen/params.gen.go \
$(P)/lint/gen/receiver.gen.go \
$(P)/lint/gen/receiver_gen_test.go &: $(P)/lint/provider.go | star
	cd $(DEVLORE_CLI) && $(CURDIR)/build/star devlore actions generate \
		--source=$(CURDIR)/$(P)/lint --gen=true --write=true --output=$(CURDIR)/$(P)/lint

$(P)/setup/gen/params.gen.go \
$(P)/setup/gen/receiver.gen.go \
$(P)/setup/gen/receiver_gen_test.go &: $(P)/setup/provider.go | star
	cd $(DEVLORE_CLI) && $(CURDIR)/build/star devlore actions generate \
		--source=$(CURDIR)/$(P)/setup --gen=true --write=true --output=$(CURDIR)/$(P)/setup

$(P)/shellcheck/gen/params.gen.go \
$(P)/shellcheck/gen/receiver.gen.go \
$(P)/shellcheck/gen/receiver_gen_test.go &: $(P)/shellcheck/provider.go | star
	cd $(DEVLORE_CLI) && $(CURDIR)/build/star devlore actions generate \
		--source=$(CURDIR)/$(P)/shellcheck --gen=true --write=true --output=$(CURDIR)/$(P)/shellcheck

GEN_PROVIDERS := \
	$(P)/commands/gen/receiver.gen.go \
	$(P)/config/gen/receiver.gen.go \
	$(P)/goast/gen/receiver.gen.go \
	$(P)/lint/gen/receiver.gen.go \
	$(P)/setup/gen/receiver.gen.go \
	$(P)/shellcheck/gen/receiver.gen.go

generate: $(GEN_PROVIDERS) ## Regenerate all provider gen/ files

##@ Extensions

build-extensions: check-wasm-target $(addsuffix -build,$(EXTENSIONS)) ## Build all WASM extensions

package-extensions: $(addsuffix -package,$(EXTENSIONS)) ## Package all extensions for distribution

clean-extensions: $(addsuffix -clean,$(EXTENSIONS)) ## Clean extension build artifacts

check-wasm-target:
	@rustup target list --installed 2>/dev/null | grep -q wasm32-wasip1 || \
		(echo "Error: wasm32-wasip1 target not installed. Run: rustup target add wasm32-wasip1" && exit 1)
