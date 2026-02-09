// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

// Package extension provides the extension loading system for star.
//
// An extension is described by a YAML specification that defines:
//   - Receivers: Go binding functions exposed to Starlark via receiver API
//   - Command: CLI subcommand implemented in Starlark
//   - Config: Configuration schema with typed fields and defaults
//   - Capabilities: Sandboxing rules for Wasm extensions
//
// # Extension Types
//
// Extensions fall into three categories:
//
//   - Binding-only: Provides receivers but no command (e.g., copyright primitives)
//   - Command-only: Provides a command but no receivers (e.g., lint.all orchestrator)
//   - Full: Provides both receivers and a command (e.g., lint.copyright)
//
// An extension MUST provide at least one receiver OR one command.
//
// # Usage
//
// Load extensions from a directory:
//
//	specs, err := extension.Discover("extensions/")
//	for _, spec := range specs {
//	    extension.Register(spec)
//	}
//
// Or use LoadAll for convenience:
//
//	err := extension.LoadAll("extensions/", "~/.star/extensions/")
//
// Query registered extensions:
//
//	spec := extension.Get("lint.copyright")
//	all := extension.All()
//
// # Extension Specification Format
//
// Example extension.yaml:
//
//	extension: lint.copyright
//	description: "Check or fix copyright headers in source files"
//
//	receivers:
//	  - name: copyright
//	    type: CopyrightChecker
//	    builtin: true
//	    functions:
//	      check: "Verify files have correct headers"
//	      fix: "Add or update headers"
//
//	command:
//	  help: "Check or fix copyright headers"
//	  implementation: lint-copyright.star
//
//	flags:
//	  - name: fix
//	    type: bool
//	    default: "false"
//	    help: Add missing headers
//
//	config:
//	  type: CopyrightConfig
//	  fields:
//	    enabled: bool
//	    license: string
//	    holder: string
//	  defaults:
//	    enabled: false
//	    license: "auto"
//
// # Wasm Extensions
//
// External extensions are distributed as WebAssembly modules. They declare
// capabilities for sandboxed execution:
//
//	receivers:
//	  - name: customlint
//	    wasm: customlint.wasm
//	    functions:
//	      analyze: "Run custom analysis"
//
//	capabilities:
//	  fs:
//	    read: ["/workspace"]
//	    write: []
//	  host_calls:
//	    - shell.run
package extension
