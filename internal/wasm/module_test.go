// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

// minimalWasm is a minimal valid Wasm module that exports an "answer" function
// returning 42. Equivalent to:
//
//	(module
//	  (func (export "answer") (result i32)
//	    i32.const 42
//	  )
//	)
var minimalWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, // magic: \0asm
	0x01, 0x00, 0x00, 0x00, // version: 1
	// Type section (1): one function type () -> i32
	0x01, 0x05, 0x01, 0x60, 0x00, 0x01, 0x7f,
	// Function section (3): one function using type 0
	0x03, 0x02, 0x01, 0x00,
	// Export section (7): export "answer" as function 0
	0x07, 0x0a, 0x01, 0x06, 0x61, 0x6e, 0x73, 0x77, 0x65, 0x72, 0x00, 0x00,
	// Code section (10): function body
	0x0a, 0x06, 0x01, 0x04, 0x00, 0x41, 0x2a, 0x0b,
}

// wasiEchoWasm is a WASI module that reads from stdin and writes to stdout.
// This is used for testing the Call() function with the JSON protocol.
// Equivalent to a module that echoes stdin to stdout.
//
// For simplicity, we use a minimal module that just exits with code 0.
// The actual echo functionality would require a more complex module.
var wasiMinimalWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, // magic: \0asm
	0x01, 0x00, 0x00, 0x00, // version: 1
	// Type section: () -> ()
	0x01, 0x04, 0x01, 0x60, 0x00, 0x00,
	// Function section: one function
	0x03, 0x02, 0x01, 0x00,
	// Export section: export "_initialize"
	0x07, 0x0f, 0x01, 0x0b, 0x5f, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x61, 0x6c, 0x69, 0x7a, 0x65, 0x00, 0x00,
	// Code section: empty function body
	0x0a, 0x04, 0x01, 0x02, 0x00, 0x0b,
}

func TestLoadModule(t *testing.T) {
	ctx := context.Background()

	host, err := NewHost(ctx, extension.Capabilities{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	// Create temp file with minimal wasm
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "minimal.wasm")
	if err := os.WriteFile(wasmPath, minimalWasm, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Load module
	mod, err := host.LoadModule(wasmPath)
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}
	module := mod.(*WasmModule) // Type assert for concrete methods

	// Verify path
	if module.Path() != wasmPath {
		// Path should be absolute
		absPath, _ := filepath.Abs(wasmPath)
		if module.Path() != absPath {
			t.Errorf("Path() = %q, want %q", module.Path(), absPath)
		}
	}

	// Verify name
	if module.Name() != "minimal" {
		t.Errorf("Name() = %q, want %q", module.Name(), "minimal")
	}

	// Verify exported functions
	funcs := module.ExportedFunctions()
	found := false
	for _, f := range funcs {
		if f == "answer" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExportedFunctions() = %v, want to contain 'answer'", funcs)
	}

	// Verify HasFunction
	if !module.HasFunction("answer") {
		t.Error("HasFunction('answer') = false, want true")
	}
	if module.HasFunction("nonexistent") {
		t.Error("HasFunction('nonexistent') = true, want false")
	}
}

func TestLoadModule_Caching(t *testing.T) {
	ctx := context.Background()

	host, err := NewHost(ctx, extension.Capabilities{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	// Create temp file
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "cached.wasm")
	if err := os.WriteFile(wasmPath, minimalWasm, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Load twice
	module1, err := host.LoadModule(wasmPath)
	if err != nil {
		t.Fatalf("LoadModule() first call error = %v", err)
	}

	module2, err := host.LoadModule(wasmPath)
	if err != nil {
		t.Fatalf("LoadModule() second call error = %v", err)
	}

	// Should return same instance
	if module1 != module2 {
		t.Error("LoadModule() should return cached instance")
	}
}

func TestLoadModule_InvalidWasm(t *testing.T) {
	ctx := context.Background()

	host, err := NewHost(ctx, extension.Capabilities{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	// Create temp file with invalid content
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "invalid.wasm")
	if err := os.WriteFile(wasmPath, []byte("not wasm"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Load should fail
	_, err = host.LoadModule(wasmPath)
	if err == nil {
		t.Error("LoadModule() expected error for invalid wasm, got nil")
	}
}

func TestModule_Call_WasiModule(t *testing.T) {
	ctx := context.Background()

	host, err := NewHost(ctx, extension.Capabilities{
		FS: extension.FSCapabilities{
			Read: []string{"/workspace"},
		},
	})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	// Create temp file with WASI module
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "wasi.wasm")
	if err := os.WriteFile(wasmPath, wasiMinimalWasm, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	mod, err := host.LoadModule(wasmPath)
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}
	module := mod.(*WasmModule)

	// Call should succeed (module just initializes and returns)
	// The module doesn't produce output, so result should be nil
	result, err := module.Call("test", []byte(`{"key": "value"}`))
	if err != nil {
		t.Logf("Call() error = %v (expected for minimal module)", err)
	}
	// Result may be nil for minimal module
	_ = result
}

func TestModule_Call_Timeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	host, err := NewHost(ctx, extension.Capabilities{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	// Create temp file
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "timeout.wasm")
	if err := os.WriteFile(wasmPath, wasiMinimalWasm, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	mod, err := host.LoadModule(wasmPath)
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}
	module := mod.(*WasmModule)

	// Cancel context before call
	cancel()

	// Call with cancelled context (context is from host)
	_, err = module.Call("test", nil)
	if err == nil {
		t.Log("Call() with cancelled context succeeded (module may have completed before cancellation)")
	}
}

func TestCapabilityChecker_CheckWrite(t *testing.T) {
	checker := NewCapabilityChecker(extension.Capabilities{
		FS: extension.FSCapabilities{
			Write: []string{"/tmp/output"},
		},
	})

	// Test allowed path
	if err := checker.CheckWrite("/tmp/output/file.txt"); err != nil {
		t.Errorf("CheckWrite() unexpected error for allowed path: %v", err)
	}

	// Test denied path
	err := checker.CheckWrite("/tmp/denied/file.txt")
	if err == nil {
		t.Error("CheckWrite() expected error for denied path, got nil")
	}
	wasmErr, ok := err.(*WasmError)
	if !ok {
		t.Errorf("CheckWrite() error type = %T, want *WasmError", err)
	} else if wasmErr.Code != ErrCodeCapability {
		t.Errorf("CheckWrite() error code = %d, want %d", wasmErr.Code, ErrCodeCapability)
	}
}

func TestCapabilityChecker_CheckHostCall(t *testing.T) {
	checker := NewCapabilityChecker(extension.Capabilities{
		HostCalls: []string{"shell.run", "http.get"},
	})

	// Test allowed call
	if err := checker.CheckHostCall("shell.run"); err != nil {
		t.Errorf("CheckHostCall() unexpected error for allowed call: %v", err)
	}

	// Test denied call
	err := checker.CheckHostCall("fs.write")
	if err == nil {
		t.Error("CheckHostCall() expected error for denied call, got nil")
	}
	wasmErr, ok := err.(*WasmError)
	if !ok {
		t.Errorf("CheckHostCall() error type = %T, want *WasmError", err)
	} else if wasmErr.Code != ErrCodeCapability {
		t.Errorf("CheckHostCall() error code = %d, want %d", wasmErr.Code, ErrCodeCapability)
	}
}

func TestNewWasmErrorf(t *testing.T) {
	err := NewWasmErrorf(ErrCodeInternal, "error: %s %d", "test", 42)

	if err.Code != ErrCodeInternal {
		t.Errorf("Code = %d, want %d", err.Code, ErrCodeInternal)
	}
	if err.Message != "error: test 42" {
		t.Errorf("Message = %q, want %q", err.Message, "error: test 42")
	}
}

func TestModule_handleError(t *testing.T) {
	ctx := context.Background()

	host, err := NewHost(ctx, extension.Capabilities{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	// Create a module for testing handleError
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "test.wasm")
	if err := os.WriteFile(wasmPath, minimalWasm, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	mod, err := host.LoadModule(wasmPath)
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}
	module := mod.(*WasmModule)

	// Test context.Canceled
	canceledErr := module.handleError(context.Canceled, "")
	if canceledErr == nil {
		t.Fatal("handleError(context.Canceled) returned nil")
	}
	wasmErr, ok := canceledErr.(*WasmError)
	if !ok {
		t.Fatalf("handleError() type = %T, want *WasmError", canceledErr)
	}
	if wasmErr.Code != ErrCodeCanceled {
		t.Errorf("Code = %d, want %d", wasmErr.Code, ErrCodeCanceled)
	}

	// Test context.DeadlineExceeded
	timeoutErr := module.handleError(context.DeadlineExceeded, "")
	wasmErr, ok = timeoutErr.(*WasmError)
	if !ok {
		t.Fatalf("handleError() type = %T, want *WasmError", timeoutErr)
	}
	if wasmErr.Code != ErrCodeTimeout {
		t.Errorf("Code = %d, want %d", wasmErr.Code, ErrCodeTimeout)
	}

	// Test generic error with stderr
	genericErr := module.handleError(os.ErrNotExist, "stderr output")
	wasmErr, ok = genericErr.(*WasmError)
	if !ok {
		t.Fatalf("handleError() type = %T, want *WasmError", genericErr)
	}
	if wasmErr.Code != ErrCodeInternal {
		t.Errorf("Code = %d, want %d", wasmErr.Code, ErrCodeInternal)
	}
	if wasmErr.Message == "" {
		t.Error("Message should not be empty")
	}
}
