// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import "fmt"

// Error codes for WasmError.
const (
	// ErrCodeExitNonZero indicates the Wasm module exited with a non-zero code.
	ErrCodeExitNonZero = 1

	// ErrCodeCanceled indicates the context was canceled.
	ErrCodeCanceled = -1

	// ErrCodeTimeout indicates the context deadline was exceeded.
	ErrCodeTimeout = -2

	// ErrCodeInternal indicates an internal error (compilation, instantiation, etc.).
	ErrCodeInternal = -3

	// ErrCodeCapability indicates a capability violation.
	ErrCodeCapability = -4

	// ErrCodeProtocol indicates a protocol error (invalid request/response).
	ErrCodeProtocol = -5
)

// WasmError represents an error from a Wasm module or the host runtime.
type WasmError struct {
	// Code is an error code. Positive codes are Wasm exit codes,
	// negative codes are host-defined (see ErrCode* constants).
	Code int `json:"code"`

	// Message is a human-readable error message.
	Message string `json:"message"`

	// Data contains optional additional error information.
	Data any `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *WasmError) Error() string {
	if e.Code == 0 {
		return e.Message
	}
	return fmt.Sprintf("wasm error %d: %s", e.Code, e.Message)
}

// NewWasmError creates a new WasmError with the given code and message.
func NewWasmError(code int, message string) *WasmError {
	return &WasmError{
		Code:    code,
		Message: message,
	}
}

// NewWasmErrorf creates a new WasmError with a formatted message.
func NewWasmErrorf(code int, format string, args ...any) *WasmError {
	return &WasmError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// CapabilityError creates a WasmError for capability violations.
func CapabilityError(action, path string) *WasmError {
	return &WasmError{
		Code:    ErrCodeCapability,
		Message: fmt.Sprintf("capability denied: cannot %s %s", action, path),
		Data:    map[string]string{"action": action, "path": path},
	}
}
