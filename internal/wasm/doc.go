// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

// Package wasm provides a WebAssembly host runtime for star extensions.
//
// The wasm package enables extensions to be distributed as WebAssembly modules,
// providing cross-platform compatibility, sandboxed execution, and near-native
// performance. It uses wazero, a pure Go Wasm runtime with no CGO dependencies.
//
// # Architecture
//
// Extensions run in a WASI sandbox with declared capabilities:
//
//   - Filesystem access is limited to explicitly granted directories
//   - No direct process spawning or network access
//   - Privileged operations require host callbacks (Phase 4)
//
// # Usage
//
// Create a host with capabilities, load a module, and call functions:
//
//	caps := extension.Capabilities{
//	    FS: extension.FSCapabilities{
//	        Read:  []string{"/workspace"},
//	        Write: []string{"/workspace"},
//	    },
//	    HostCalls: []string{"shell.run"},
//	}
//
//	host, err := wasm.NewHost(ctx, caps)
//	if err != nil {
//	    return err
//	}
//	defer host.Close()
//
//	module, err := host.LoadModule("extension.wasm")
//	if err != nil {
//	    return err
//	}
//
//	result, err := module.Call(ctx, "check", argsJSON)
//
// # Data Passing
//
// Functions receive arguments and return results as JSON via stdin/stdout.
// The Request/Response protocol wraps function calls:
//
//	Request:  {"id": 1, "method": "check", "params": {...}}
//	Response: {"id": 1, "result": {...}} or {"id": 1, "error": {...}}
//
// # Module Lifecycle
//
// Modules are compiled once and cached. Each Call() instantiates a fresh
// module instance (reactor pattern) to ensure clean state.
//
// # Phase 2 vs Phase 4
//
// Phase 2 (this implementation) provides:
//   - wazero host setup with compilation caching
//   - Capability validation
//   - Module loading and caching
//   - Basic function calling via stdin/stdout
//
// Phase 4 will add:
//   - Host callbacks (shell.run, http.get, fs.*)
//   - Direct memory access for performance-critical paths
package wasm
