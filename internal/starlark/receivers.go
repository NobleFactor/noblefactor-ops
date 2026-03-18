// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

// Receiver singletons — these have no per-runtime dependencies.
// JSON, YAML, Regexp, UI, and star* providers are framework-managed via StarlarkRuntime.
// Schema and StarlarkParse are replaced by json/yaml.Resource.Validate and star* providers.
var (
	// Shellcheck provides shell script analysis operations.
	Shellcheck = NewShellcheckReceiver()

	// Config provides configuration operations.
	Config = NewConfigReceiver()
)
