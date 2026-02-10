// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package extension

// WasmHost defines the interface for the WebAssembly runtime.
// Implemented by internal/wasm/host.go.
//
// The host is responsible for:
//   - Loading and caching compiled Wasm modules
//   - Executing functions within modules
//   - Enforcing capability restrictions
//   - Providing host callbacks for privileged operations
type WasmHost interface {
	// LoadModule compiles and instantiates a Wasm module from a file.
	// The module is cached for subsequent calls.
	LoadModule(wasmPath string) (WasmModule, error)

	// Close releases all resources held by the host.
	Close() error
}

// WasmModule represents a loaded WebAssembly module.
type WasmModule interface {
	// Name returns the module name.
	Name() string

	// Functions returns the list of exported function names.
	Functions() []string

	// Call invokes a function in this module.
	// Arguments and return values are serialized as bytes (e.g., JSON).
	Call(function string, args []byte) ([]byte, error)
}

// WasmError represents an error from the Wasm runtime.
type WasmError struct {
	Op      string // Operation that failed
	Module  string // Module name if applicable
	Message string // Error message
}

func (e *WasmError) Error() string {
	if e.Module != "" {
		return e.Op + " " + e.Module + ": " + e.Message
	}
	return e.Op + ": " + e.Message
}

// Common Wasm errors.
var (
	// ErrModuleNotFound is returned when a Wasm module file doesn't exist.
	ErrModuleNotFound = &WasmError{Op: "load", Message: "module not found"}

	// ErrFunctionNotFound is returned when a function doesn't exist in a module.
	ErrFunctionNotFound = &WasmError{Op: "call", Message: "function not found"}
)
