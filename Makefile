# SPDX-License-Identifier: MIT
# Copyright (c) 2025-2026 Noble Factor. All rights reserved.

SHELL := bash
.SHELLFLAGS := -o errexit -o nounset -o pipefail -c
.ONESHELL:
.SILENT:

## PARAMETERS

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"

### STAR

# Code generator (star binary from this repo).
STAR ?= bin/star

# devlore-cli repo (star loads devlore extensions from here for codegen).
DEVLORE_CLI ?= ../devlore-cli

## VARIABLES (static)

# Provider source root.
P := internal/starlark/provider

# Rustup toolchain bin directory (avoids MacPorts/Homebrew conflicts)
RUSTUP_BIN := $(shell rustup run stable rustc --print sysroot 2>/dev/null)/bin

# Extensions list - populated by included build.mk files
EXTENSIONS :=

# Include all extension build rules
-include extensions/*/build.mk

## TARGETS

.PHONY: all build clean test install deps check-codegen
.PHONY: build-extensions package-extensions clean-extensions
.PHONY: check-wasm-target star generate generate-register help

##@ Help

HELP_COLWIDTH ?= 24

help: ## Show available targets
	awk 'BEGIN {FS = ":.*##"; pad = $(HELP_COLWIDTH); print "Usage: make <target> [VAR=VALUE]"; print ""; print "Targets:"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "  %-*s %s\n", pad, $$1, $$2} /^##@/ {printf "\n%s\n", substr($$0,5)}' $(MAKEFILE_LIST)

##@ Build

all: build-extensions build

star: ## Build the star code generator
	go build $(LDFLAGS) -o bin/star ./cmd/star

build: generate ## Build star binary
	go build $(LDFLAGS) -o bin/star ./cmd/star

clean: clean-extensions ## Remove build artifacts, generated files, and gen/ directories
	rm -rf bin/
	rm -f $(P)/register.go
	find $(P) -type d -name gen -exec rm -rf {} +

##@ Test

test: generate ## Run Go tests
	go test ./...

##@ Quality

# Install binary to GOBIN or ~/.local/bin
install: build
	mkdir -p $(or $(GOBIN),$(HOME)/.local/bin)
	cp bin/star $(or $(GOBIN),$(HOME)/.local/bin)/
	echo "Installed to $(or $(GOBIN),$(HOME)/.local/bin)"

# Verify all dependencies are available
deps:
	go mod download
	go mod verify

# Verify generated provider files match LKG codegen output
CODEGEN_BRANCH ?= develop
check-codegen:
	scripts/check-codegen.sh --branch=$(CODEGEN_BRANCH)

##@ Extensions

# Build all extensions (calls each extension's -build target)
build-extensions: check-wasm-target $(addsuffix -build,$(EXTENSIONS))

# Package all extensions for distribution
package-extensions: $(addsuffix -package,$(EXTENSIONS))

# Clean all extension build artifacts
clean-extensions: $(addsuffix -clean,$(EXTENSIONS))

# Check that wasm32-wasip1 target is installed
check-wasm-target:
	rustup target list --installed 2>/dev/null | grep -q wasm32-wasip1 || \
		(echo "Error: wasm32-wasip1 target not installed. Run: rustup target add wasm32-wasip1" && exit 1)

##@ Code Generation

# Each grouped target (&:) fires one star invocation that produces all gen files.
# Generation runs only when provider.go is newer than the gen outputs.
#
# All local providers use access=immediate: receiver + receiver_gen_test + params.

$(P)/commands/gen/params.gen.go \
$(P)/commands/gen/receiver.gen.go \
$(P)/commands/gen/receiver_gen_test.go &: $(P)/commands/provider.go | star
	cd $(DEVLORE_CLI) && $(CURDIR)/$(STAR) devlore actions generate --source=$(CURDIR)/$(P)/commands --gen=true --write=true --output=$(CURDIR)/$(P)/commands

$(P)/config/gen/params.gen.go \
$(P)/config/gen/receiver.gen.go \
$(P)/config/gen/receiver_gen_test.go &: $(P)/config/provider.go | star
	cd $(DEVLORE_CLI) && $(CURDIR)/$(STAR) devlore actions generate --source=$(CURDIR)/$(P)/config --gen=true --write=true --output=$(CURDIR)/$(P)/config

$(P)/goast/gen/params.gen.go \
$(P)/goast/gen/receiver.gen.go \
$(P)/goast/gen/receiver_gen_test.go &: $(P)/goast/provider.go | star
	cd $(DEVLORE_CLI) && $(CURDIR)/$(STAR) devlore actions generate --source=$(CURDIR)/$(P)/goast --gen=true --write=true --output=$(CURDIR)/$(P)/goast

$(P)/lint/gen/params.gen.go \
$(P)/lint/gen/receiver.gen.go \
$(P)/lint/gen/receiver_gen_test.go &: $(P)/lint/provider.go | star
	cd $(DEVLORE_CLI) && $(CURDIR)/$(STAR) devlore actions generate --source=$(CURDIR)/$(P)/lint --gen=true --write=true --output=$(CURDIR)/$(P)/lint

$(P)/setup/gen/params.gen.go \
$(P)/setup/gen/receiver.gen.go \
$(P)/setup/gen/receiver_gen_test.go &: $(P)/setup/provider.go | star
	cd $(DEVLORE_CLI) && $(CURDIR)/$(STAR) devlore actions generate --source=$(CURDIR)/$(P)/setup --gen=true --write=true --output=$(CURDIR)/$(P)/setup

$(P)/shellcheck/gen/params.gen.go \
$(P)/shellcheck/gen/receiver.gen.go \
$(P)/shellcheck/gen/receiver_gen_test.go &: $(P)/shellcheck/provider.go | star
	cd $(DEVLORE_CLI) && $(CURDIR)/$(STAR) devlore actions generate --source=$(CURDIR)/$(P)/shellcheck --gen=true --write=true --output=$(CURDIR)/$(P)/shellcheck

GEN_PROVIDERS := \
	$(P)/commands/gen/receiver.gen.go \
	$(P)/config/gen/receiver.gen.go \
	$(P)/goast/gen/receiver.gen.go \
	$(P)/lint/gen/receiver.gen.go \
	$(P)/setup/gen/receiver.gen.go \
	$(P)/shellcheck/gen/receiver.gen.go

generate-register: $(GEN_PROVIDERS) ## Generate provider register.go with blank imports
	echo '// Code generated by make generate-register; DO NOT EDIT.' > $(P)/register.go
	echo '' >> $(P)/register.go
	echo '// Package provider triggers init() in all provider packages via blank imports.' >> $(P)/register.go
	echo '// Importing this package causes every provider to call op.AnnounceReceiver(),' >> $(P)/register.go
	echo '// making them available via op.Receivers().' >> $(P)/register.go
	echo 'package provider' >> $(P)/register.go
	echo '' >> $(P)/register.go
	echo 'import (' >> $(P)/register.go
	echo '	_ "github.com/NobleFactor/devlore-cli/pkg/op/provider"' >> $(P)/register.go
	for dir in $$(find $(P) -type d -name gen | sort); do
		pkg=$$(echo $$dir | sed 's|^|github.com/NobleFactor/noblefactor-ops/|')
		echo "	_ \"$$pkg\"" >> $(P)/register.go
	done
	echo ')' >> $(P)/register.go

generate: generate-register ## Run all code generation
