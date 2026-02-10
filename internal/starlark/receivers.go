// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"go.starlark.net/starlark"
)

// Receiver instances - these are singletons used across all Starlark executions.
var (
	// File provides file system operations (renamed from fs).
	File = NewFileReceiver()

	// JSON provides JSON encoding/decoding operations.
	JSON = NewJSONReceiver()

	// YAML provides YAML encoding/decoding operations.
	YAML = NewYAMLReceiver()

	// Schema provides JSON Schema validation operations.
	Schema = NewSchemaReceiver()

	// Shell provides shell script analysis operations.
	Shell = NewShellReceiver()

	// Regexp provides regular expression operations with pattern caching.
	Regexp = NewRegexpReceiver()

	// Go provides Go source parsing operations.
	Go = NewGoReceiver()

	// Lint provides static analysis operations.
	Lint = NewLintReceiver()

	// Setup provides repository setup operations.
	Setup = NewSetupReceiver()

	// Config provides configuration operations.
	Config = NewConfigReceiver()

	// StarlarkParse provides Starlark source parsing operations.
	StarlarkParse = NewStarlarkParseReceiver()
)

// Receivers returns a StringDict containing all receiver instances.
// This is used by Runtime.buildPredeclared() to inject receivers into scripts.
func Receivers() starlark.StringDict {
	return starlark.StringDict{
		"file":           File,
		"json":           JSON,
		"yaml":           YAML,
		"schema":         Schema,
		"shell":          Shell,
		"regexp":         Regexp,
		"go":             Go,
		"lint":           Lint,
		"setup":          Setup,
		"config":         Config,
		"starlark_parse": StarlarkParse,
	}
}
