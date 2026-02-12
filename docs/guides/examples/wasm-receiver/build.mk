# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.
#
# Build rules for com.noblefactor.star.Gitignore extension
# Included by top-level Makefile

EXT_GITIGNORE := com.noblefactor.star.Gitignore
EXT_GITIGNORE_DIR := extensions/$(EXT_GITIGNORE)
EXT_GITIGNORE_WASM := $(EXT_GITIGNORE_DIR)/receivers/gitignore.wasm

# Register this extension
EXTENSIONS += $(EXT_GITIGNORE)

# Rust sources for dependency tracking
EXT_GITIGNORE_SRCS := $(wildcard $(EXT_GITIGNORE_DIR)/src/*.rs) $(EXT_GITIGNORE_DIR)/Cargo.toml

# Build WASM receiver (reactor mode — exports _initialize, not _start)
$(EXT_GITIGNORE)-build: $(EXT_GITIGNORE_WASM)

$(EXT_GITIGNORE_WASM): $(EXT_GITIGNORE_SRCS)
	@echo "Building: $(EXT_GITIGNORE)"
	@mkdir -p $(dir $@)
	cd $(EXT_GITIGNORE_DIR) && PATH="$(RUSTUP_BIN):$$PATH" cargo build --target wasm32-wasip1 --release
	cp $(EXT_GITIGNORE_DIR)/target/wasm32-wasip1/release/gitignore.wasm $@

# Package extension for distribution
$(EXT_GITIGNORE)-package: $(EXT_GITIGNORE)-build
	@echo "Packaging: $(EXT_GITIGNORE)"
	@mkdir -p $(EXT_GITIGNORE_DIR)/dist
	cd $(EXT_GITIGNORE_DIR) && zip -r dist/$(EXT_GITIGNORE).star-ext \
		extension.yaml \
		receivers/*.wasm

# Clean build artifacts
$(EXT_GITIGNORE)-clean:
	@echo "Cleaning: $(EXT_GITIGNORE)"
	cd $(EXT_GITIGNORE_DIR) && cargo clean 2>/dev/null || true
	rm -rf $(EXT_GITIGNORE_DIR)/dist

.PHONY: $(EXT_GITIGNORE)-build $(EXT_GITIGNORE)-package $(EXT_GITIGNORE)-clean
