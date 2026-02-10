// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

// Package ignore provides gitignore-aware file filtering.
//
// This package uses the gitignore WASM extension (BurntSushi's ignore crate)
// when available, falling back to a Go stub implementation otherwise.
//
// The WASM module is set by the runtime during extension loading via SetModule.
package ignore

import (
	"encoding/json"
	"path/filepath"
	"sync"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

// module holds the gitignore WASM module set by the runtime.
var (
	mu     sync.RWMutex
	module extension.WasmModule
)

// SetModule sets the gitignore WASM module.
// Called by the runtime after loading the gitignore extension.
func SetModule(m extension.WasmModule) {
	mu.Lock()
	defer mu.Unlock()
	module = m
}

// getModule returns the current WASM module, if set.
func getModule() extension.WasmModule {
	mu.RLock()
	defer mu.RUnlock()
	return module
}

// Matcher checks if paths should be ignored based on .gitignore rules.
type Matcher struct {
	base string
}

// New creates a new Matcher for the given directory.
func New(dir string) (*Matcher, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	return &Matcher{base: absDir}, nil
}

// Match checks if the given path should be ignored.
func (m *Matcher) Match(path string) bool {
	if mod := getModule(); mod != nil {
		return matchWasm(mod, path, m.base)
	}
	return matchStub(path, m.base)
}

// Filter returns paths that are NOT ignored.
func (m *Matcher) Filter(paths []string) []string {
	if len(paths) == 0 {
		return paths
	}

	if mod := getModule(); mod != nil {
		return filterWasm(mod, paths, m.base)
	}
	return filterStub(paths, m.base)
}

// filterParams is the input for the WASM filter method.
type filterParams struct {
	Paths []string `json:"paths"`
	Base  string   `json:"base"`
}

// filterResult is the output from the WASM filter method.
type filterResult struct {
	Paths []string `json:"paths"`
}

// matchesParams is the input for the WASM matches method.
type matchesParams struct {
	Path string `json:"path"`
	Base string `json:"base"`
}

// matchesResult is the output from the WASM matches method.
type matchesResult struct {
	Ignored bool `json:"ignored"`
}

// filterWasm calls the WASM module to filter paths.
func filterWasm(mod extension.WasmModule, paths []string, base string) []string {
	params := filterParams{Paths: paths, Base: base}
	input, err := json.Marshal(params)
	if err != nil {
		return filterStub(paths, base)
	}

	output, err := mod.Call("filter", input)
	if err != nil {
		return filterStub(paths, base)
	}

	var result filterResult
	if err := json.Unmarshal(output, &result); err != nil {
		return filterStub(paths, base)
	}

	return result.Paths
}

// matchWasm calls the WASM module to check if a path is ignored.
func matchWasm(mod extension.WasmModule, path, base string) bool {
	params := matchesParams{Path: path, Base: base}
	input, err := json.Marshal(params)
	if err != nil {
		return matchStub(path, base)
	}

	output, err := mod.Call("matches", input)
	if err != nil {
		return matchStub(path, base)
	}

	var result matchesResult
	if err := json.Unmarshal(output, &result); err != nil {
		return matchStub(path, base)
	}

	return result.Ignored
}
