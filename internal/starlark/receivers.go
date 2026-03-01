// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

// Receiver singletons — these have no per-runtime dependencies.
var (
	// JSON provides JSON encoding/decoding operations.
	JSON = NewJSONReceiver()

	// YAML provides YAML encoding/decoding operations.
	YAML = NewYAMLReceiver()

	// Schema provides JSON Schema validation operations.
	Schema = NewSchemaReceiver()

	// Shellcheck provides shell script analysis operations.
	Shellcheck = NewShellcheckReceiver()

	// Regexp provides regular expression operations with pattern caching.
	Regexp = NewRegexpReceiver()

	// Go provides Go source parsing operations.
	Go = NewGoReceiver()

	// Config provides configuration operations.
	Config = NewConfigReceiver()

	// StarlarkParse provides Starlark source parsing operations.
	StarlarkParse = NewStarlarkParseReceiver()
)
