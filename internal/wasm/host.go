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
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

// WasmHost manages the wazero runtime and module lifecycle.
type WasmHost struct {
	runtime wazero.Runtime
	cache   wazero.CompilationCache
	caps    extension.Capabilities
	checker *CapabilityChecker
	ctx     context.Context

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

	return &WasmHost{
		runtime: rt,
		cache:   cache,
		caps:    caps,
		checker: NewCapabilityChecker(caps),
		ctx:     ctx,
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
func (h *WasmHost) LoadModule(wasmPath string) (*WasmModule, error) {
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

	module := &WasmModule{
		path:     absPath,
		compiled: compiled,
		host:     h,
	}

	// Store in cache (may race, but that's okay - both are valid)
	actual, _ := h.modules.LoadOrStore(absPath, module)
	return actual.(*WasmModule), nil
}

// Capabilities returns the host's declared capabilities.
func (h *WasmHost) Capabilities() extension.Capabilities {
	return h.caps
}

// Checker returns the capability checker for this host.
func (h *WasmHost) Checker() *CapabilityChecker {
	return h.checker
}

// Close releases all resources held by the host.
// This closes all cached modules, the runtime, and the compilation cache.
func (h *WasmHost) Close() error {
	var errs []error

	// Close all cached modules
	h.modules.Range(func(key, value any) bool {
		if m, ok := value.(*WasmModule); ok {
			if err := m.compiled.Close(h.ctx); err != nil {
				errs = append(errs, fmt.Errorf("close module %s: %w", key, err))
			}
		}
		return true
	})

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
