// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

// WasmHost manages the wazero runtime and module lifecycle.
type WasmHost struct {
	runtime    wazero.Runtime
	cache      wazero.CompilationCache
	caps       extension.Capabilities
	checker    *CapabilityChecker
	callbacks  HostCallbacks
	hostModule api.Module
	ctx        context.Context

	// modules caches compiled modules by absolute path.
	modules sync.Map // map[string]*WasmModule
}

// NewHost creates a new WasmHost with the given capabilities.
// The host uses wazero as the WebAssembly runtime with compilation caching
// for improved performance on repeated module loads.
func NewHost(ctx context.Context, caps extension.Capabilities) (*WasmHost, error) {
	// Validate capabilities
	if err := ValidateCapabilities(caps); err != nil {
		return nil, fmt.Errorf("invalid capabilities: %w", err)
	}

	// Create cache directory
	cacheDir, err := ensureCacheDir()
	if err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	// Create compilation cache
	cache, err := wazero.NewCompilationCacheWithDir(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("create compilation cache: %w", err)
	}

	// Create runtime with cache
	config := wazero.NewRuntimeConfig().
		WithCompilationCache(cache).
		WithCloseOnContextDone(true)

	rt := wazero.NewRuntimeWithConfig(ctx, config)

	// Instantiate WASI for modules that need it
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		closeErr := cache.Close(ctx)
		rtErr := rt.Close(ctx)
		if closeErr != nil || rtErr != nil {
			return nil, fmt.Errorf("instantiate WASI: %w (cleanup errors: cache=%v, rt=%v)", err, closeErr, rtErr)
		}
		return nil, fmt.Errorf("instantiate WASI: %w", err)
	}

	// Create capability checker and callbacks
	checker := NewCapabilityChecker(caps)
	callbacks := NewDefaultCallbacks(checker)

	// Instantiate host module for callbacks
	hostModule, err := InstantiateHostModule(ctx, rt, callbacks)
	if err != nil {
		_ = cache.Close(ctx)
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("instantiate host module: %w", err)
	}

	return &WasmHost{
		runtime:    rt,
		cache:      cache,
		caps:       caps,
		checker:    checker,
		callbacks:  callbacks,
		hostModule: hostModule,
		ctx:        ctx,
	}, nil
}

// ensureCacheDir creates and returns the cache directory for compiled modules.
func ensureCacheDir() (string, error) {
	userCache, err := os.UserCacheDir()
	if err != nil {
		userCache = os.TempDir()
	}
	cacheDir := filepath.Join(userCache, "star", "wasm", "wazero")
	return cacheDir, os.MkdirAll(cacheDir, 0755)
}

// LoadModule compiles and caches a Wasm module from the given path.
// Subsequent calls with the same path return the cached module.
// Returns extension.WasmModule to satisfy the extension.WasmHost interface.
func (h *WasmHost) LoadModule(wasmPath string) (extension.WasmModule, error) {
	// Get absolute path for cache key
	absPath, err := filepath.Abs(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	// Check cache first
	if cached, ok := h.modules.Load(absPath); ok {
		return cached.(*WasmModule), nil
	}

	// Read and compile
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("read wasm: %w", err)
	}

	compiled, err := h.runtime.CompileModule(h.ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile wasm: %w", err)
	}

	// Validate reactor contract: all modules loaded by star must be WASI reactors
	if _, ok := compiled.ExportedFunctions()["_initialize"]; !ok {
		_ = compiled.Close(h.ctx)
		return nil, fmt.Errorf("wasm module %s: missing required export '_initialize' (reactor mode)", wasmPath)
	}
	if _, ok := compiled.ExportedMemories()["memory"]; !ok {
		_ = compiled.Close(h.ctx)
		return nil, fmt.Errorf("wasm module %s: missing required export 'memory'", wasmPath)
	}

	module := &WasmModule{
		path:     absPath,
		compiled: compiled,
		host:     h,
	}

	// Store in cache (may race, but that's okay - both are valid)
	actual, _ := h.modules.LoadOrStore(absPath, module)
	return actual.(*WasmModule), nil
}

// Call invokes a function in the given module.
// Implements extension.WasmHost interface.
func (h *WasmHost) Call(module extension.WasmModule, function string, args []byte) ([]byte, error) {
	return module.Call(function, args)
}

// Capabilities returns the host's declared capabilities.
func (h *WasmHost) Capabilities() extension.Capabilities {
	return h.caps
}

// Checker returns the capability checker for this host.
func (h *WasmHost) Checker() *CapabilityChecker {
	return h.checker
}

// Callbacks returns the host callbacks for this host.
func (h *WasmHost) Callbacks() HostCallbacks {
	return h.callbacks
}

// Close releases all resources held by the host.
// This closes all cached modules, the host module, the runtime, and the compilation cache.
func (h *WasmHost) Close() error {
	var errs []error

	// Close all cached modules (and their persistent instances)
	h.modules.Range(func(key, value any) bool {
		if m, ok := value.(*WasmModule); ok {
			// Close reactor instance first (if cached)
			if err := m.closeInstance(); err != nil {
				errs = append(errs, fmt.Errorf("close instance %s: %w", key, err))
			}
			if err := m.compiled.Close(h.ctx); err != nil {
				errs = append(errs, fmt.Errorf("close module %s: %w", key, err))
			}
		}
		return true
	})

	// Close host module
	if h.hostModule != nil {
		if err := h.hostModule.Close(h.ctx); err != nil {
			errs = append(errs, fmt.Errorf("close host module: %w", err))
		}
	}

	// Close runtime
	if err := h.runtime.Close(h.ctx); err != nil {
		errs = append(errs, fmt.Errorf("close runtime: %w", err))
	}

	// Close cache
	if err := h.cache.Close(h.ctx); err != nil {
		errs = append(errs, fmt.Errorf("close cache: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
