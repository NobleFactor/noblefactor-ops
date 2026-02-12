// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/sys"
)

// infrastructureExports are WASM exports that are part of the runtime
// infrastructure and should not be exposed as callable functions.
var infrastructureExports = map[string]bool{
	"_initialize": true,
	"_start":      true,
	"alloc":       true,
	"dealloc":     true,
	"memory":      true, // Won't appear in ExportedFunctions but guard anyway
}

// WasmModule represents a compiled Wasm module.
type WasmModule struct {
	path     string
	compiled wazero.CompiledModule
	host     *WasmHost

	// Persistent instance reused across calls via alloc/dealloc shared memory.
	mu       sync.Mutex
	instance api.Module // nil until first call
}

// Call invokes a named function export on a persistent module instance.
// The function signature convention is: fn(ptr i32, len i32) -> packed_result i64
// where the return is (result_ptr << 32 | result_len).
func (m *WasmModule) Call(function string, args []byte) ([]byte, error) {
	instance, err := m.getInstance()
	if err != nil {
		return nil, err
	}

	fn := instance.ExportedFunction(function)
	if fn == nil {
		return nil, NewWasmErrorf(ErrCodeProtocol, "function %q not exported", function)
	}

	// Write args to module memory via alloc
	argPtr, argLen, err := m.writeToMemory(instance, args)
	if err != nil {
		return nil, NewWasmErrorf(ErrCodeInternal, "write args: %v", err)
	}

	// Call the function: fn(ptr, len) -> packed(result_ptr, result_len)
	results, err := fn.Call(m.host.ctx, uint64(argPtr), uint64(argLen))
	if err != nil {
		// Instance may be in undefined state after trap — discard it
		m.closeInstance()
		return nil, m.handleError(err)
	}

	// Free the input args
	if freeErr := m.wasmFree(instance, argPtr, argLen); freeErr != nil {
		return nil, NewWasmErrorf(ErrCodeInternal, "free args: %v", freeErr)
	}

	if len(results) == 0 {
		return nil, nil
	}

	// Unpack result: high 32 bits = ptr, low 32 bits = len
	packed := results[0]
	resultPtr := uint32(packed >> 32)
	resultLen := uint32(packed & 0xFFFFFFFF)

	if resultLen == 0 {
		return nil, nil
	}

	// Read result from module memory
	result, err := m.readFromMemory(instance, resultPtr, resultLen)
	if err != nil {
		return nil, NewWasmErrorf(ErrCodeInternal, "read result: %v", err)
	}

	// Free the result pointer
	if freeErr := m.wasmFree(instance, resultPtr, resultLen); freeErr != nil {
		return nil, NewWasmErrorf(ErrCodeInternal, "free result: %v", freeErr)
	}

	return result, nil
}

// getInstance returns the persistent instance, creating it on first call.
// The instance is mutex-protected for safe concurrent use.
func (m *WasmModule) getInstance() (api.Module, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.instance != nil {
		return m.instance, nil
	}

	// Configure instance without stdin/stdout (reactor setup only)
	config := wazero.NewModuleConfig().
		WithName("").
		WithStartFunctions("_initialize")

	// Apply filesystem capabilities
	config, err := m.applyFSConfig(config)
	if err != nil {
		return nil, err
	}

	// Create host state and context
	hostState := NewHostState(m.host.callbacks)
	callCtx := withHostState(m.host.ctx, hostState)

	instance, err := m.host.runtime.InstantiateModule(callCtx, m.compiled, config)
	if err != nil {
		return nil, m.handleError(err)
	}

	m.instance = instance
	return instance, nil
}

// closeInstance closes any cached reactor instance.
func (m *WasmModule) closeInstance() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.instance == nil {
		return nil
	}
	err := m.instance.Close(m.host.ctx)
	m.instance = nil
	return err
}

// wasmAlloc calls the module's alloc(size) -> ptr export.
func (m *WasmModule) wasmAlloc(instance api.Module, size uint32) (uint32, error) {
	allocFn := instance.ExportedFunction("alloc")
	if allocFn == nil {
		return 0, fmt.Errorf("module does not export 'alloc'")
	}
	results, err := allocFn.Call(m.host.ctx, uint64(size))
	if err != nil {
		return 0, fmt.Errorf("alloc(%d): %w", size, err)
	}
	if len(results) == 0 {
		return 0, fmt.Errorf("alloc(%d): no return value", size)
	}
	return uint32(results[0]), nil
}

// wasmFree calls the module's dealloc(ptr, size) export.
func (m *WasmModule) wasmFree(instance api.Module, ptr, size uint32) error {
	freeFn := instance.ExportedFunction("dealloc")
	if freeFn == nil {
		return fmt.Errorf("module does not export 'dealloc'")
	}
	_, err := freeFn.Call(m.host.ctx, uint64(ptr), uint64(size))
	return err
}

// writeToMemory allocates space in module memory and writes data into it.
func (m *WasmModule) writeToMemory(instance api.Module, data []byte) (ptr, size uint32, err error) {
	if len(data) == 0 {
		return 0, 0, nil
	}

	size = uint32(len(data))
	ptr, err = m.wasmAlloc(instance, size)
	if err != nil {
		return 0, 0, err
	}

	mem := instance.Memory()
	if mem == nil {
		return 0, 0, fmt.Errorf("module has no exported memory")
	}

	if !mem.Write(ptr, data) {
		return 0, 0, fmt.Errorf("write to memory at offset %d (size %d) failed", ptr, size)
	}

	return ptr, size, nil
}

// readFromMemory reads data from module memory.
func (m *WasmModule) readFromMemory(instance api.Module, ptr, size uint32) ([]byte, error) {
	mem := instance.Memory()
	if mem == nil {
		return nil, fmt.Errorf("module has no exported memory")
	}

	data, ok := mem.Read(ptr, size)
	if !ok {
		return nil, fmt.Errorf("read from memory at offset %d (size %d) failed", ptr, size)
	}

	// Copy the data — wazero memory may be reused
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
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
func (m *WasmModule) handleError(err error) error {
	// Check for WASI exit
	var exitErr *sys.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ExitCode() == 0 {
			return nil // Normal exit
		}
		return &WasmError{
			Code:    int(exitErr.ExitCode()),
			Message: fmt.Sprintf("wasm exited with code %d", exitErr.ExitCode()),
		}
	}

	// Check for context errors
	if errors.Is(err, context.Canceled) {
		return NewWasmError(ErrCodeCanceled, "execution canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return NewWasmError(ErrCodeTimeout, "execution timeout")
	}

	return NewWasmError(ErrCodeInternal, err.Error())
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

// Functions implements extension.WasmModule interface.
// It returns exported function names with infrastructure exports filtered out.
// Infrastructure exports (_initialize, _start, alloc, dealloc) are internal to the
// WASM runtime protocol and are not user-callable functions.
func (m *WasmModule) Functions() []string {
	exports := m.compiled.ExportedFunctions()
	names := make([]string, 0, len(exports))
	for name := range exports {
		if !infrastructureExports[name] {
			names = append(names, name)
		}
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
