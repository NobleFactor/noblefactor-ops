// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package extension

// WasmHost defines the interface for the WebAssembly runtime.
// Worker 5 will implement this interface in internal/wasm/host.go.
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

	// Call invokes a function in a loaded module.
	// Arguments and return values are serialized as bytes (e.g., JSON or MessagePack).
	Call(module WasmModule, function string, args []byte) ([]byte, error)

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

// wasmHost is the registered Wasm runtime.
// Set by internal/wasm package during initialization.
var wasmHost WasmHost

// SetWasmHost registers the Wasm runtime.
// Called by internal/wasm during init.
func SetWasmHost(host WasmHost) {
	wasmHost = host
}

// GetWasmHost returns the registered Wasm host.
// Returns nil if no host is registered.
func GetWasmHost() WasmHost {
	return wasmHost
}

// HasWasmSupport returns true if a Wasm runtime is available.
func HasWasmSupport() bool {
	return wasmHost != nil
}

// CallWasmReceiver invokes a function on a Wasm receiver.
// This is a convenience function for calling receiver methods.
//
// Example:
//
//	result, err := CallWasmReceiver(spec, "check", argsJSON)
func CallWasmReceiver(receiver *ReceiverSpec, function string, args []byte) ([]byte, error) {
	if !HasWasmSupport() {
		return nil, ErrNoWasmRuntime
	}

	if receiver.Builtin {
		return nil, ErrNotWasmReceiver
	}

	module, err := wasmHost.LoadModule(receiver.Wasm)
	if err != nil {
		return nil, err
	}

	return wasmHost.Call(module, function, args)
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
	// ErrNoWasmRuntime is returned when no Wasm runtime is registered.
	ErrNoWasmRuntime = &WasmError{Op: "wasm", Message: "no runtime registered"}

	// ErrNotWasmReceiver is returned when trying to call a builtin receiver via Wasm.
	ErrNotWasmReceiver = &WasmError{Op: "wasm", Message: "receiver is not a Wasm module"}

	// ErrModuleNotFound is returned when a Wasm module file doesn't exist.
	ErrModuleNotFound = &WasmError{Op: "load", Message: "module not found"}

	// ErrFunctionNotFound is returned when a function doesn't exist in a module.
	ErrFunctionNotFound = &WasmError{Op: "call", Message: "function not found"}
)
