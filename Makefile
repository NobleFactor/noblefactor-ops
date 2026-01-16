# SPDX-License-Identifier: MIT
# Copyright (c) 2025 Noble Factor. All rights reserved.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"

.PHONY: all build clean test install

all: build

build:
	go build $(LDFLAGS) -o bin/nf-ops ./cmd/nf-ops

clean:
	rm -rf bin/

test:
	go test ./...

# Install binary to GOBIN or ~/.local/bin
install: build
	@mkdir -p $(or $(GOBIN),$(HOME)/.local/bin)
	cp bin/nf-ops $(or $(GOBIN),$(HOME)/.local/bin)/
	@echo "Installed to $(or $(GOBIN),$(HOME)/.local/bin)"

# Verify all dependencies are available
deps:
	go mod download
	go mod verify
