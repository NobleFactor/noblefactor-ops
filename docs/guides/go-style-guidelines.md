# Go Style Guidelines

## 1. File Layout (MANDATORY)

Every Go file, in exact order:

1. **Copyright header** — `// SPDX-License-Identifier: SSPL-1.0` + `// Copyright (c) 2025-2026 Noble Factor. All rights reserved.`
2. **Package declaration** — optional package-level doc comment (as in `helpers.go`: `// Package file provides...`)
3. **Imports** — stdlib block, blank line, internal packages block
4. **`init()`** — if present (e.g., registering constructors)
5. **Interface guards** — `var _ op.ContextProvider = (*Provider)(nil)` with `// Interface Guard:` comment
6. **Package-level vars** — grouped `var ()` blocks with doc comments
7. **Main struct** — with doc comment and annotations
8. **Exported types** — e.g., `type Reducer func(...)` with doc comment
9. **Main struct methods** — exported first, then unexported (see Method Region Hierarchy)
10. **All other structs and their methods** — alphabetical order, each following the same exported-then-unexported pattern

## 2. Method Region Hierarchy

Every struct's methods use a two-level region hierarchy. Top-level regions are **ALL CAPS**; sub-regions are **Sentence case**. Empty sub-regions are omitted.

```go
// region EXPORTED METHODS

// region State management
// (getters, setters — methods that read or write struct state)
// endregion

// region Behaviors
// (everything else — operations, queries, transformations)
// endregion

// endregion

// region UNEXPORTED METHODS

// region State management
// endregion

// region Behaviors
// endregion

// endregion
```

## 3. Delineators Within Behaviors

Within a `Behaviors` sub-region, plain comment lines (not regions) group methods by kind. An example of this kind of grouping follows:

| Delineator            | Meaning                                                   | Signature pattern                            |
| --------------------- | --------------------------------------------------------- | -------------------------------------------- |
| `// Fallible actions` | Operations that can fail                                  | Returns `error` (alone or with other values) |
| `// Actions`          | Infallible operations — pure computation or thin wrappers | No `error` in return                         |

Not every struct needs both. A struct where all methods are of one kind needs no delineators at all.

Within each delineator group, methods are **alphabetical**.

### Sort Order Summary

1. **State management** before **Behaviors**
2. Within Behaviors: **Fallible** → **Actions**
3. Within each group: **alphabetical**
4. **Exported** before **Unexported** (same sub-structure in both)

## 4. Doc Comment Format

Every exported method (and unexported helpers):

```go
// Name <present-tense verb phrase>.
//
// Optional extended description.
//
// Parameters:
//   - paramName: Description (default: value)
//
// Returns:
//   - type: description
//
// +devlore:defaults key=value  (if applicable)
```

- The first line is the summary: it begins with the identifier name, then a present-tense verb phrase
  (`Instance returns …`, `Copy copies …`), on **one line**, filled to column 120. Never a bare imperative —
  Go's `golint` requires the leading name.
- The optional extended description is **zero or more paragraphs**, each filled to column 120 and separated by a
  blank `//` line.
- Parameters and Returns sections always present on exported methods
- Default values documented inline
- `+devlore:defaults` directives (when applicable) follow the Returns section
- Unexported helpers also get full doc comments with Parameters/Returns

## 5. Struct Design

Type members are documented inline with the member, not in the type's doc comment:

```go
// Resource represents a handle to data that can be streamed.
type Resource struct {
	op.ResourceBase
	SourcePath SourcePath  // path used for all I/O
	Checksum   string      // "sha256:<hex>" computed at resolve time
}
```

**Resource:**

- Embeds `op.ResourceBase`
- All fields exported (value type, passed by value)
- `NewResource(path)` — pure computation, no I/O, no error
- `Resolve()` — performs I/O, populates metadata
- `String()` delegates to `r.Format(r)`

## 6. Naming Conventions

| Category                  | Pattern                                 | Examples                           |
| ------------------------- | --------------------------------------- | ---------------------------------- |
| Exported action methods   | Verb or VerbNoun                        | `Backup`, `Copy`, `WriteBytes`     |
| Query methods             | `Is`/`Exists` returning `(bool, error)` | `Exists`, `IsDir`, `IsFile`        |
| Utility methods           | Short noun/verb, return `string`        | `Join`, `Name`, `Parent`           |
| Unexported methods        | camelCase verbs                         | `compensateWrite`, `prepareWrite`  |
| Unexported free functions | camelCase verbs                         | `isDirAndNotEmpty`, `checksumFile` |

## 7. Error Handling

- Return **zero-value structs** on error: `return Resource{}, err`
- Check errors **immediately** after the producing call
- Use `errors.Is(err, os.ErrNotExist)` for specific discrimination
- Graceful degradation annotated with `//nolint:nilerr // reason`
- **Fix every issue as discovered** — never pass by warnings or errors

## 8. Code Formatting

- **Line width**: wrap at column 120. No exceptions for documentation. Function signatures may exceed 120 when
  breaking would hurt readability.
- **Blank line after function signature** before the body
- **No blank line before `return`** in short functions
- **Blank lines** separate logical blocks within a function
- **`defer f.Close()`** immediately after the open call
- **Octal literals** use `0o` prefix: `0o644`, `0o750`
- **No legacy/backward-compat code** — greenfield; delete when in doubt
- **No unnecessary comments, docstrings, or type annotations** on unchanged code

## 9. Test Style

- **Helper function** at top with `t.Helper()`
- **Section separators**: `// --- ActionName ---`
- **Test naming**: `TestAction_ScenarioDescription`
- **Setup**: `t.TempDir()`, `t.Cleanup()` — no manual temp dir management
- **Assertions**: direct `if` checks with `t.Fatalf` / `t.Errorf` — no assertion library
- **Test after each change** to verify fixes
- **Use `make test`** — never bare `go test`

## 10. Helpers File

- Separate `helpers.go` for **unexported free functions** (not methods)
- Each has full doc comments with Parameters/Returns
- No region markers — the file itself is the organizational unit
- Package doc comment can live here on the `package` line

---

## Appendix A: Provider Development (`pkg/op/provider/*/provider.go`)

This appendix covers conventions specific to devlore operation providers. These build on the generic guidelines above.

### A.1 Provider Struct

- Embeds `op.ProviderBase`
- Annotated with `+devlore:access=both` (or appropriate access level)
- No exported fields — behavior via methods only

```go
// +devlore:access=both
type Provider struct {
	op.ProviderBase
}
```

### A.2 Tombstone Struct

Each provider defines a Tombstone that holds compensation state:

- Embeds `op.TombstoneBase`
- Domain-specific fields documented inline with the member

```go
// Tombstone holds file-specific compensation state.
type Tombstone struct {
	op.TombstoneBase
	RecoveryPath string // where data was moved during the operation; empty means nothing to recover
}
```

### A.3 Compensable Pair Convention

Providers implement compensable actions — a forward method paired with its undo:

```go
func (p *Provider) Action(args...) (result T, undo Tombstone, err error) { ... }
func (p *Provider) CompensateAction(undo Tombstone) error { ... }
```

- Forward returns three values: `(result, undo, error)` — always
- Compensate takes the tombstone, returns `error`
- The pair is **adjacent** — `CompensateX` immediately follows `X`
- Compensate methods open with a nil-resource guard: `if undo.Resource() == nil { return nil }`
- Naming: `Compensate` + action name (e.g., `CompensateBackup`, `CompensateCopy`)

### A.4 Provider Behavior Delineators

Providers add a `// Compensable actions` delineator before the generic ones. The full sort order within a provider's Behaviors region:

| Delineator               | Meaning                             | Signature pattern                                             |
| ------------------------ | ----------------------------------- | ------------------------------------------------------------- |
| `// Compensable actions` | Forward action paired with its undo | `(result T, undo U, err error)` + `CompensateX(undo U) error` |
| `// Fallible actions`    | Operations that can fail            | Returns `error` (alone or with other values)                  |
| `// Actions`             | Infallible operations               | No `error` in return                                          |

Within each group, methods are **alphabetical** — except compensable pairs, where `CompensateX` immediately follows `X`.

### A.5 Example: `pkg/op/provider/file/provider.go`

This example shows the **region structure and method ordering** only. The [§4](#4-doc-comment-format) doc-comment
blocks are elided from the exported methods to keep the layout legible — but every method shown carries a full §4
block (summary + `Parameters:` / `Returns:`), **exported and unexported alike**, as the unexported region at the
bottom demonstrates in full. Unexported methods are **not** exempt from §4.

```go
// region EXPORTED METHODS

// region State management

func (p *Provider) Root() string { ... }

// endregion

// region Behaviors

// Compensable actions

func (p *Provider) Backup(...)    (Resource, Tombstone, error) { ... }
func (p *Provider) CompensateBackup(undo Tombstone) error      { ... }

func (p *Provider) Copy(...)      (Resource, Tombstone, error) { ... }
func (p *Provider) CompensateCopy(undo Tombstone) error        { ... }

// ... Link, Move, Remove, RemoveAll, Unlink, WalkTree, WriteBytes, WriteText
// ... each followed immediately by its CompensateX

// Fallible actions

func (p *Provider) Exists(resource Resource) (bool, error)           { ... }
func (p *Provider) Glob(pattern string, ...) ([]string, error)       { ... }
func (p *Provider) IsDir(resource Resource) (bool, error)            { ... }
func (p *Provider) IsFile(resource Resource) (bool, error)           { ... }
func (p *Provider) Mkdir(resource Resource, ...) (Resource, error)   { ... }
func (p *Provider) Read(path Resource) (Resource, error)             { ... }

// Actions

func (p *Provider) Join(parts ...string) string { ... }
func (p *Provider) Name(path string) string     { ... }
func (p *Provider) Parent(path string) string   { ... }

// endregion

// endregion

// region UNEXPORTED METHODS

// region Behaviors

// compensateWrite restores the bytes displaced by the paired forward write from the tombstone.
//
// Parameters:
//   - `undo`: the tombstone captured by the forward write.
//
// Returns:
//   - `error`: non-nil when the displaced bytes cannot be restored.
func (p *Provider) compensateWrite(undo Tombstone) error { ... }

// prepareWrite stages a write, capturing the bytes it would displace into a tombstone for compensation.
//
// Parameters:
//   - `resource`: the target resource to write.
//
// Returns:
//   - `Resource`: the staged resource.
//   - `Tombstone`: the compensation state for the paired compensateWrite.
//   - `error`: non-nil when staging fails.
func (p *Provider) prepareWrite(resource Resource) (Resource, Tombstone, error) { ... }

// pruneEmptyParents removes now-empty parent directories of `path`, stopping at `boundary`.
//
// Parameters:
//   - `path`: the path whose parents are candidates for pruning.
//   - `prune`: whether pruning is enabled; false makes this a no-op.
//   - `boundary`: the directory at which pruning stops.
func (p *Provider) pruneEmptyParents(path string, prune bool, boundary string) { ... }

// write performs the staged write, returning the written resource and its compensation tombstone.
//
// Parameters:
//   - `resource`: the target resource to write.
//
// Returns:
//   - `Resource`: the written resource.
//   - `Tombstone`: the compensation state.
//   - `error`: non-nil when the write fails.
func (p *Provider) write(resource Resource, ...) (Resource, Tombstone, error) { ... }

// endregion

// endregion
```
