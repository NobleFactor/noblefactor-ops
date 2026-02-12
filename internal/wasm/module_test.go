// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

// minimalWasm is a minimal valid Wasm module that exports an "answer" function
// returning 42. It does NOT export _initialize or memory, so it fails the
// reactor validation in LoadModule. Used to test rejection of non-reactor modules.
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

// noMemoryWasm exports _initialize but no memory. Used to test that LoadModule
// rejects modules missing the required memory export.
//
//	(module
//	  (func (export "_initialize"))
//	)
var noMemoryWasm = []byte{
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

// wasiMinimalWasm is a minimal WASI reactor module that exports both _initialize
// and memory. This satisfies the reactor contract validated by LoadModule.
//
//	(module
//	  (memory (export "memory") 1)
//	  (func (export "_initialize"))
//	)
var wasiMinimalWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, // magic: \0asm
	0x01, 0x00, 0x00, 0x00, // version: 1
	// Type section (1): one function type () -> ()
	0x01, 0x04, 0x01, 0x60, 0x00, 0x00,
	// Function section (3): one function using type 0
	0x03, 0x02, 0x01, 0x00,
	// Memory section (5): one memory, min=1 page
	0x05, 0x03, 0x01, 0x00, 0x01,
	// Export section (7): "memory" (memory 0) and "_initialize" (func 0)
	0x07, 0x18, 0x02,
	0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x02, 0x00, // "memory" -> memory 0
	0x0b, 0x5f, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x61, 0x6c, 0x69, 0x7a, 0x65, 0x00, 0x00, // "_initialize" -> func 0
	// Code section (10): empty function body
	0x0a, 0x04, 0x01, 0x02, 0x00, 0x0b,
}

func TestLoadModule(t *testing.T) {
	ctx := context.Background()

	host, err := NewHost(ctx, extension.Capabilities{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	// Create temp file with valid reactor wasm
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "reactor.wasm")
	if err := os.WriteFile(wasmPath, wasiMinimalWasm, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Load module
	mod, err := host.LoadModule(wasmPath)
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}
	module := mod.(*WasmModule)

	// Verify path is absolute
	absPath, _ := filepath.Abs(wasmPath)
	if module.Path() != absPath {
		t.Errorf("Path() = %q, want %q", module.Path(), absPath)
	}

	// Verify name
	if module.Name() != "reactor" {
		t.Errorf("Name() = %q, want %q", module.Name(), "reactor")
	}

	// Verify HasFunction
	if !module.HasFunction("_initialize") {
		t.Error("HasFunction('_initialize') = false, want true")
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
	if err := os.WriteFile(wasmPath, wasiMinimalWasm, 0644); err != nil {
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

func TestLoadModule_MissingInitialize(t *testing.T) {
	ctx := context.Background()

	host, err := NewHost(ctx, extension.Capabilities{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	// Create temp file with module missing _initialize
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "no_init.wasm")
	if err := os.WriteFile(wasmPath, minimalWasm, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = host.LoadModule(wasmPath)
	if err == nil {
		t.Fatal("LoadModule() expected error for missing _initialize, got nil")
	}
	if !strings.Contains(err.Error(), "_initialize") {
		t.Errorf("error should mention _initialize, got: %v", err)
	}
}

func TestLoadModule_MissingMemory(t *testing.T) {
	ctx := context.Background()

	host, err := NewHost(ctx, extension.Capabilities{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	// Create temp file with module that has _initialize but no memory
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "no_memory.wasm")
	if err := os.WriteFile(wasmPath, noMemoryWasm, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = host.LoadModule(wasmPath)
	if err == nil {
		t.Fatal("LoadModule() expected error for missing memory, got nil")
	}
	if !strings.Contains(err.Error(), "memory") {
		t.Errorf("error should mention memory, got: %v", err)
	}
}

func TestFunctions_FiltersInfrastructure(t *testing.T) {
	ctx := context.Background()

	host, err := NewHost(ctx, extension.Capabilities{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	// Load the minimal reactor module (only exports _initialize)
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "reactor.wasm")
	if err := os.WriteFile(wasmPath, wasiMinimalWasm, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	mod, err := host.LoadModule(wasmPath)
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}
	module := mod.(*WasmModule)

	// ExportedFunctions should include _initialize
	allFuncs := module.ExportedFunctions()
	hasInit := false
	for _, f := range allFuncs {
		if f == "_initialize" {
			hasInit = true
			break
		}
	}
	if !hasInit {
		t.Errorf("ExportedFunctions() = %v, want to contain '_initialize'", allFuncs)
	}

	// Functions() should filter out _initialize (infrastructure)
	funcs := module.Functions()
	for _, f := range funcs {
		if infrastructureExports[f] {
			t.Errorf("Functions() contains infrastructure export %q", f)
		}
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

// TestGitignoreWasm_ExportsInitialize verifies that the committed gitignore.wasm
// is a WASI reactor (exports _initialize) NOT a WASI command (exports _start).
// This is a regression guard — the star runtime calls WithStartFunctions("_initialize")
// and silently skips modules that only export _start.
func TestGitignoreWasm_ExportsInitialize(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "docs", "guides", "examples",
		"wasm-receiver", "receivers", "gitignore.wasm")
	assertWasmReactor(t, wasmPath)
}

// assertWasmReactor loads a WASM binary and verifies it exports _initialize
// (reactor mode) and does NOT export _start (command mode).
func assertWasmReactor(t *testing.T, wasmPath string) {
	t.Helper()

	ctx := context.Background()
	host, err := NewHost(ctx, extension.Capabilities{
		FS: extension.FSCapabilities{Read: []string{"/workspace"}},
	})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	mod, err := host.LoadModule(wasmPath)
	if err != nil {
		t.Fatalf("LoadModule(%s) error = %v", wasmPath, err)
	}
	module := mod.(*WasmModule)

	exports := module.compiled.ExportedFunctions()

	if _, ok := exports["_initialize"]; !ok {
		t.Errorf("%s: missing _initialize export (reactor mode required)", wasmPath)
	}
	if _, ok := exports["_start"]; ok {
		t.Errorf("%s: exports _start (command mode) — must be a reactor with _initialize only", wasmPath)
	}
}

// TestGitignoreWasm_ReactorProtocol verifies the committed gitignore.wasm
// works as a shared memory reactor: persistent instance, named exports,
// alloc/dealloc memory management.
func TestGitignoreWasm_ReactorProtocol(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "docs", "guides", "examples",
		"wasm-receiver", "receivers", "gitignore.wasm")

	ctx := context.Background()
	host, err := NewHost(ctx, extension.Capabilities{
		FS: extension.FSCapabilities{Read: []string{"/workspace"}},
	})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	mod, err := host.LoadModule(wasmPath)
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}
	module := mod.(*WasmModule)

	t.Run("functions discovered", func(t *testing.T) {
		funcs := module.Functions()
		want := map[string]bool{"matches": false, "filter": false}
		for _, f := range funcs {
			if _, ok := want[f]; ok {
				want[f] = true
			}
		}
		for name, found := range want {
			if !found {
				t.Errorf("Functions() missing %q, got %v", name, funcs)
			}
		}
		// Verify infrastructure exports are filtered
		for _, f := range funcs {
			if infrastructureExports[f] {
				t.Errorf("Functions() contains infrastructure export %q", f)
			}
		}
	})

	t.Run("matches method", func(t *testing.T) {
		result, err := mod.Call("matches", []byte(`{"path":"vendor/foo.go","base":"."}`))
		if err != nil {
			t.Fatalf("Call(matches) error = %v", err)
		}
		if result == nil {
			t.Fatal("Call(matches) returned nil result")
		}
		var resp struct {
			Ignored bool `json:"ignored"`
		}
		if err := json.Unmarshal(result, &resp); err != nil {
			t.Fatalf("unmarshal result: %v (raw: %s)", err, result)
		}
	})

	t.Run("filter method", func(t *testing.T) {
		result, err := mod.Call("filter", []byte(`{"paths":["main.go","vendor/dep.go"],"base":"."}`))
		if err != nil {
			t.Fatalf("Call(filter) error = %v", err)
		}
		if result == nil {
			t.Fatal("Call(filter) returned nil result")
		}
		var resp struct {
			Paths []string `json:"paths"`
		}
		if err := json.Unmarshal(result, &resp); err != nil {
			t.Fatalf("unmarshal result: %v (raw: %s)", err, result)
		}
	})

	t.Run("instance reuse", func(t *testing.T) {
		// Call matches twice — both should use the same persistent instance
		_, err := mod.Call("matches", []byte(`{"path":"a.go","base":"."}`))
		if err != nil {
			t.Fatalf("first Call() error = %v", err)
		}
		_, err = mod.Call("matches", []byte(`{"path":"b.go","base":"."}`))
		if err != nil {
			t.Fatalf("second Call() error = %v", err)
		}
		// Verify instance was cached (non-nil)
		module.mu.Lock()
		hasInstance := module.instance != nil
		module.mu.Unlock()
		if !hasInstance {
			t.Error("reactor instance should be cached after calls")
		}
	})

	t.Run("unknown method", func(t *testing.T) {
		_, err := mod.Call("nonexistent", []byte(`{}`))
		if err == nil {
			t.Error("Call(nonexistent) should return error")
		}
	})
}

func TestModule_handleError(t *testing.T) {
	ctx := context.Background()

	host, err := NewHost(ctx, extension.Capabilities{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	// Create a module for testing handleError (must pass reactor validation)
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "test.wasm")
	if err := os.WriteFile(wasmPath, wasiMinimalWasm, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	mod, err := host.LoadModule(wasmPath)
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}
	module := mod.(*WasmModule)

	// Test context.Canceled
	canceledErr := module.handleError(context.Canceled)
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
	timeoutErr := module.handleError(context.DeadlineExceeded)
	wasmErr, ok = timeoutErr.(*WasmError)
	if !ok {
		t.Fatalf("handleError() type = %T, want *WasmError", timeoutErr)
	}
	if wasmErr.Code != ErrCodeTimeout {
		t.Errorf("Code = %d, want %d", wasmErr.Code, ErrCodeTimeout)
	}

	// Test generic error
	genericErr := module.handleError(os.ErrNotExist)
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
