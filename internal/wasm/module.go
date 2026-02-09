// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/sys"
)

// WasmModule represents a compiled Wasm module.
type WasmModule struct {
	path     string
	compiled wazero.CompiledModule
	host     *WasmHost
}

// Call invokes a function in the module with JSON-encoded arguments.
// Arguments are passed via stdin as a JSON Request, and results are
// read from stdout as a JSON Response.
//
// Each call creates a fresh module instance (reactor pattern) to ensure
// clean state between invocations.
func (m *WasmModule) Call(ctx context.Context, function string, args []byte) ([]byte, error) {
	var stdout, stderr bytes.Buffer

	// Build request envelope
	req := Request{
		ID:     1, // Single request per instance, no need for correlation
		Method: function,
		Params: args,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, NewWasmErrorf(ErrCodeProtocol, "marshal request: %v", err)
	}

	// Configure module instance
	config := wazero.NewModuleConfig().
		WithName(""). // Anonymous - allows multiple instances
		WithStdin(bytes.NewReader(reqBytes)).
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs(filepath.Base(m.path), function).
		WithStartFunctions("_initialize") // Reactor mode - don't run _start

	// Apply filesystem capabilities
	config, err = m.applyFSConfig(config)
	if err != nil {
		return nil, err
	}

	// Instantiate and run
	result, err := m.host.runtime.InstantiateModule(ctx, m.compiled, config)
	if err != nil {
		// DO NOT call result.Close() here - wazero already cleaned up on error
		return nil, m.handleError(err, stderr.String())
	}
	defer result.Close(ctx)

	// Check stderr for errors
	if stderr.Len() > 0 {
		return nil, NewWasmErrorf(ErrCodeInternal, "wasm stderr: %s", stderr.String())
	}

	// Handle empty stdout (module may not have produced output)
	if stdout.Len() == 0 {
		return nil, nil
	}

	// Parse response envelope
	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, NewWasmErrorf(ErrCodeProtocol, "unmarshal response: %v", err)
	}

	// Check for error in response
	if resp.Error != nil {
		return nil, resp.Error
	}

	return resp.Result, nil
}

// applyFSConfig configures filesystem access based on capabilities.
func (m *WasmModule) applyFSConfig(config wazero.ModuleConfig) (wazero.ModuleConfig, error) {
	caps := m.host.caps
	fsConfig := wazero.NewFSConfig()

	// Apply read-only mounts
	for _, path := range caps.FS.Read {
		expanded := expandPath(path)
		resolved, err := filepath.Abs(expanded)
		if err != nil {
			return nil, NewWasmErrorf(ErrCodeCapability, "resolve read path %s: %v", path, err)
		}
		// Mount at the original path so extensions see consistent paths
		fsConfig = fsConfig.WithReadOnlyDirMount(resolved, expanded)
	}

	// Apply read-write mounts (these override read-only for the same path)
	for _, path := range caps.FS.Write {
		expanded := expandPath(path)
		resolved, err := filepath.Abs(expanded)
		if err != nil {
			return nil, NewWasmErrorf(ErrCodeCapability, "resolve write path %s: %v", path, err)
		}
		fsConfig = fsConfig.WithDirMount(resolved, expanded)
	}

	return config.WithFSConfig(fsConfig), nil
}

// handleError converts wazero errors to WasmError.
func (m *WasmModule) handleError(err error, stderrContent string) error {
	// Check for WASI exit
	var exitErr *sys.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ExitCode() == 0 {
			return nil // Normal exit
		}
		msg := fmt.Sprintf("wasm exited with code %d", exitErr.ExitCode())
		if stderrContent != "" {
			msg += ": " + stderrContent
		}
		return &WasmError{Code: int(exitErr.ExitCode()), Message: msg}
	}

	// Check for context errors
	if errors.Is(err, context.Canceled) {
		return NewWasmError(ErrCodeCanceled, "execution canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return NewWasmError(ErrCodeTimeout, "execution timeout")
	}

	// Generic error
	msg := err.Error()
	if stderrContent != "" {
		msg += ": " + stderrContent
	}
	return NewWasmError(ErrCodeInternal, msg)
}

// Path returns the absolute filesystem path to this module.
func (m *WasmModule) Path() string {
	return m.path
}

// Name returns the module name (filename without extension).
func (m *WasmModule) Name() string {
	base := filepath.Base(m.path)
	ext := filepath.Ext(base)
	return base[:len(base)-len(ext)]
}

// ExportedFunctions returns the names of functions exported by this module.
// The list is sorted alphabetically.
func (m *WasmModule) ExportedFunctions() []string {
	exports := m.compiled.ExportedFunctions()
	names := make([]string, 0, len(exports))
	for name := range exports {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HasFunction checks if the module exports a function with the given name.
func (m *WasmModule) HasFunction(name string) bool {
	exports := m.compiled.ExportedFunctions()
	_, ok := exports[name]
	return ok
}
