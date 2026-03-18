// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

// Receiver singletons — these have no per-runtime dependencies.
// JSON, YAML, Regexp, UI, file, shellcheck, and star* providers are framework-managed via StarlarkRuntime.
var (
	// Config provides configuration operations.
	Config = NewConfigReceiver()
)
