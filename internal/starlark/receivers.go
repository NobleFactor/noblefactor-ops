// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

// Receiver singletons — these have no per-runtime dependencies.
// JSON, YAML, Regexp, and UI are now framework-managed via StarlarkRuntime.
var (
	// Schema provides JSON Schema validation operations.
	Schema = NewSchemaReceiver()

	// Shellcheck provides shell script analysis operations.
	Shellcheck = NewShellcheckReceiver()

	// Go provides Go source parsing operations.
	Go = NewGoReceiver()

	// Config provides configuration operations.
	Config = NewConfigReceiver()

	// StarlarkParse provides Starlark source parsing operations.
	StarlarkParse = NewStarlarkParseReceiver()
)
