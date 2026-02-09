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
//   - Privileged operations require host callbacks
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
// # Host Callbacks
//
// Wasm modules can invoke privileged host operations by importing the "star_host"
// module. The following callbacks are available:
//
//   - shell_run: Execute shell commands (requires "shell.run" capability)
//   - http_get: Make HTTP GET requests (requires "http.get" capability)
//   - fs_read: Read files (requires "fs.read" capability and path permission)
//   - fs_write: Write files (requires "fs.write" capability and path permission)
//   - get_last_error: Retrieve error messages after failed callbacks
//
// All callbacks validate capabilities before execution. The callback functions
// use a JSON-over-memory protocol where requests and responses are passed via
// linear memory pointers.
//
// # Callback Protocol
//
// Host functions accept request data via memory pointers and return response
// data to a provided buffer:
//
//	shell_run(req_ptr, req_len, resp_ptr, resp_cap) -> resp_len
//	http_get(req_ptr, req_len, resp_ptr, resp_cap) -> resp_len
//	fs_read(req_ptr, req_len, resp_ptr, resp_cap) -> resp_len
//	fs_write(req_ptr, req_len) -> error_code
//
// Return values >= 0 indicate success (response length), negative values are
// error codes. Use get_last_error to retrieve the error message.
//
// # Error Codes
//
// Host functions return these error codes on failure:
//
//   - -1: Context canceled
//   - -2: Operation timeout
//   - -3: Internal error
//   - -4: Capability violation
//   - -5: Protocol error (invalid JSON, buffer too small)
package wasm
