---
title: "Star CLI Syntax Cleanup"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/121
status: complete
created: 2026-03-22
updated: 2026-03-22
---

# Plan: Star CLI Syntax Cleanup

## Summary

Fix the star CLI so that boolean flags behave as bools, integer flags behave as
ints, path arguments are positional and variadic, file discovery uniformly
respects `.gitignore`, and the unnecessary `--tests` flag is removed.

## Goals

1. **Type-correct flags**: `--fix` is a boolean toggle, `--indent` is an integer — native types in both cobra and starlark
2. **Positional args**: `star lint go-style [flags] [path ...]` instead of `--path`
3. **Uniform path handling**: All commands process paths the same way, including `lint all`
4. **Gitignore everywhere**: File discovery in receivers uses `ignore.WalkTree()` instead of raw `filepath.WalkDir()`
5. **Remove dead weight**: Drop `--tests` from `lint go-style`

## Current State

| Issue | Status | Impact |
| --- | --- | --- |
| Bool flags rendered as `--fix string` | Broken | Every bool flag across all commands |
| Int flags rendered as `--indent string` | Broken | `lint shell` |
| `--path` is a named flag | Wrong | 5 lint commands |
| `--tests` on go-style | Unnecessary | No Go tool treats test files differently for style |
| `lint all` has no path arg | Missing | Cannot scope `lint all` to a subdirectory |
| Receivers use raw `filepath.WalkDir` | Gap | `lint markdown`, `lint shell` ignore `.gitignore` |

Root cause (flags): `registerStarlarkCommand()` in `cmd/star/main.go:340` registers
every flag with `cobraCmd.Flags().String()` regardless of declared type. The `Flag`
struct in `internal/starlark/command.go` has no `Type` field, so type information from
`FlagSpec` is discarded at load time.

Root cause (gitignore): `findMarkdownFiles()` and `CollectShellFiles()` in the lint
receiver use `filepath.WalkDir` with hardcoded directory excludes instead of
`ignore.WalkTree()` from `internal/ignore/`.

## Requirements

### Desired CLI Behavior

**Boolean flags** — toggle style, no value required:

```bash
star lint go-style --fix           # fix mode on
star lint go-style                 # fix mode off (default)
star lint go-style --fix=false     # explicit off (free with cobra)
```

**Integer flags** — numeric value:

```bash
star lint shell --indent 2         # parsed as int
star lint shell --indent=2         # also works
```

**Positional args** — zero or more paths, defaulting to `.` (or `./...` for Go):

```bash
star lint go-style                 # scans "."
star lint go-style ./internal      # scans one directory
star lint go-style cmd/ internal/  # scans two directories
star lint go ./...                 # Go package pattern (default)
star lint go ./cmd/... ./pkg/...   # multiple Go package patterns
star lint go ./internal            # single Go package (not recursive)
star lint all ./internal           # all linters scoped to ./internal
```

**Starlark consumption** — native types, not string comparison:

```python
# Before
fix_mode = ctx.args.get("fix", "false") == "true"
indent = int(ctx.args.get("indent", "4"))
scan_path = ctx.args.get("path", ".")

# After
fix_mode = ctx.args.get("fix", False)
indent = ctx.args.get("indent", 4)
paths = ctx.args.get("path", ["."])
```

### Proposed CLI Syntax (All Commands)

| Command | Syntax |
| --- | --- |
| `lint all` | `star lint all [--fix] [path ...]` |
| `lint copyright` | `star lint copyright [--fix] [path ...]` |
| `lint go` | `star lint go [--config FILE] [--skip-mod-tidy] [path ...]` |
| `lint go-style` | `star lint go-style [--fix] [--verbose] [path ...]` |
| `lint markdown` | `star lint markdown [--fix] [path ...]` |
| `lint shell` | `star lint shell [--severity LVL] [--indent N] [path ...]` |
| `lint tools` | `star lint tools` (unchanged) |

### Extension YAML Changes

Before:

```yaml
flags:
  - name: fix
    type: bool
    default: "false"
    help: Rewrite files to fix violations
  - name: path
    type: string
    default: "."
    help: Root directory to scan
  - name: tests
    type: bool
    default: "true"
    help: Include test files (_test.go)
  - name: verbose
    type: bool
    default: "false"
    help: Print each file as it is checked
```

After:

```yaml
args:
  - name: path
    help: Paths to scan (files or directories)
    default: "."
    variadic: true
flags:
  - name: fix
    type: bool
    default: "false"
    help: Rewrite files to fix violations
  - name: verbose
    type: bool
    default: "false"
    help: Print each file as it is checked
```

Note: `default` in `FlagSpec` stays as a string in the YAML — the Go code parses it
based on `type`. No YAML schema change needed for existing defaults.

### Decisions Made

- **`--config` on `lint go`**: Stays as a flag — it's a config file path, not a target.
- **`--tests` on `lint go-style`**: Removed — no Go tool treats test files differently for style/formatting.
- **`lint go` path normalization**: None. Paths are passed to `golangci-lint` as-is. `./internal` means one package, `./internal/...` means recursive. Standard Go convention.
- **`lint all` path forwarding**: `lint all` gains `[path ...]` and forwards to each sibling via `cmd.run(fix=fix_mode, path=paths)`. Each sibling interprets paths for its own file types. `lint tools` ignores paths.
- **Gitignore in receivers**: Receivers use `ignore.WalkTree()` for file discovery instead of raw `filepath.WalkDir()`. `lint go` is exempt — `golangci-lint` operates on Go package paths, and `.gitignore`d Go files are nonexistent in practice.
- **Native starlark types**: `ctx.args` carries `starlark.Bool` and `starlark.Int` for typed flags, `starlark.List` for variadic positional args. The `.star` files use native types instead of string comparison.

## Implementation Phases

### Phase 1: Type-Aware Flag Registration — `complete`

Carry flag type through from YAML to cobra so bools render as bools and ints as ints.
Pass native types to starlark — `starlark.Bool` for bools, `starlark.Int` for ints.
Update all `.star` files that consume typed flags.

- [x] Add `Type` field to `Flag` struct (`internal/starlark/command.go`)
- [x] Propagate `FlagSpec.Type` → `Flag.Type` in `loadExtensionCommand` (`internal/starlark/runtime.go`)
- [x] Update `registerStarlarkCommand` (`cmd/star/main.go`) to register flags by type
- [x] `Command.Run` builds starlark dict with native types via `flagToStarlark()` helper; `map[string]string` signature and `CommandTree` interface unchanged
- [x] Update all `.star` files to use native types (noblefactor-ops + devlore-cli)
- [x] Verify: `star lint go-style --help` shows `--fix` without `string` type annotation
- [x] Verify: `star lint shell --help` shows `--indent int` (not `--indent string`)
- [x] All tests pass

**Files**:

- `internal/starlark/command.go` — Add `Type` to `Flag`, update `Run` signature/dict building
- `internal/starlark/runtime.go` — Propagate type, update `RunCommand`
- `cmd/star/main.go` — Type-switch in registration and collection
- `internal/provider/commands/provider.go` — Update if `RunCommand` changes
- `internal/provider/commands/types.go` — Update `CommandTree` interface if needed
- `star/extensions/*/commands/*.star` — All files with bool/int flag consumption (6 files)

### Phase 2: Positional Args Infrastructure — `complete`

Add an `args` concept to the extension spec and wire it through to cobra and starlark.

- [x] Add `ArgSpec` struct to `internal/extension/spec.go` with validation
- [x] Add `Arg` struct and `Args` field to `Command` in `internal/starlark/command.go`
- [x] Propagate `ArgSpec` → `Arg` in `loadExtensionCommand`
- [x] Update `registerStarlarkCommand`: Use string with arg placeholders, `cobra.ArbitraryArgs` / `cobra.NoArgs`
- [x] `Command.Run` maps positional args to named `ctx.args` entries (variadic → `starlark.List`)
- [x] `CommandTree.RunCommand` uses variadic `...string` — no nil needed at call sites
- [x] `CommandRef.run()` separates list kwargs (positional) from scalar kwargs (flags)
- [x] Commands without arg specs reject positional args (`cobra.NoArgs`)
- [x] All tests pass

**Files**:

- `internal/extension/spec.go` — `ArgSpec`, validation
- `internal/starlark/command.go` — `Arg` struct, `Command.Args`, updated `Run`
- `internal/starlark/runtime.go` — Propagation, `CommandTree` impl
- `cmd/star/main.go` — Cobra arg registration
- `internal/provider/commands/provider.go` — Positional arg forwarding
- `internal/provider/commands/types.go` — `CommandTree` interface

### Phase 3: Extension Updates — `complete`

Convert all `--path` flags to positional args, remove `--tests`, add path to `lint all`.
Update `.star` files to iterate over path lists.

- [x] **lint.go-style**: Remove `--tests` and `--path`; add variadic `path` arg; update starlark to iterate paths
- [x] **lint.copyright**: Convert to variadic `path` arg; update `collect_source_files` to iterate
- [x] **lint.markdown**: Convert to variadic `path` arg; aggregate results across paths
- [x] **lint.shell**: Convert to variadic `path` arg; aggregate results across paths
- [x] **lint.go**: Convert to variadic `path` arg; change receiver `Go(paths []string, ...)` to accept multiple paths in one golangci-lint invocation
- [x] **lint.all**: Add variadic `path` arg; forward `path=paths` to siblings; `lint.tools` skips paths
- [x] All `--help` shows `[path ...]` syntax, `--tests` removed, `--path` removed
- [x] All tests pass

**Files per extension** (2 each):

- `star/extensions/com.noblefactor.star.<Ext>/extension.yaml`
- `star/extensions/com.noblefactor.star.<Ext>/commands/<cmd>.star`

### Phase 4: Uniform Gitignore in Receivers — `complete`

Move file discovery to starlark via `file.find()` (respects `.gitignore`), change
receivers to accept `[]string` file lists instead of paths.

- [x] `Markdown(path string, fix bool)` → `Markdown(files []string, fix bool)`; removed `findMarkdownFiles()`
- [x] `Shell(path string, severity string, indent int)` → `Shell(files []string, severity string, indent int)`; starlark discovers files
- [x] Added `file` receiver to LintMarkdown and LintShell extensions
- [x] Starlark commands use `file.find()` for discovery, pass file lists to receivers
- [x] Removed `internal/ignore/` — redundant with `devlore-cli/pkg/op/provider/file/gitignore`
- [x] `lint go` — no change (operates on Go package paths)
- [x] `lint go-style`, `lint copyright` — no change (already use `file.find()`)
- [x] All tests pass

## Files to Create/Modify

| File | Action | Phase | Purpose |
| --- | --- | --- | --- |
| `internal/starlark/command.go` | Modify | 1, 2 | Add `Type` to Flag, native type args, add `Arg` struct |
| `internal/starlark/runtime.go` | Modify | 1, 2 | Propagate types and args from spec |
| `internal/extension/spec.go` | Modify | 2 | Add `ArgSpec`, validation |
| `cmd/star/main.go` | Modify | 1, 2 | Type-aware registration, positional arg support |
| `internal/provider/commands/provider.go` | Modify | 1, 2 | RunCommand signature, positional arg forwarding |
| `internal/provider/commands/types.go` | Modify | 1, 2 | CommandTree interface |
| 6 `.star` files | Modify | 1 | Native bool/int consumption |
| 6 `extension.yaml` files | Modify | 3 | Convert path flags to args, remove --tests |
| 6 `.star` files | Modify | 3 | Iterate over path lists |
| `internal/provider/lint/provider.go` | Modify | 4 | `findMarkdownFiles()` → `ignore.WalkTree()` |
| `internal/provider/shellcheck/*.go` | Modify | 4 | `CollectShellFiles()` → `ignore.WalkTree()` |
| `internal/provider/goast/*.go` | Modify | 5 | Fix package comment extra blank line |

### Phase 5: Bug Fixes — `complete`

Fix bugs discovered during this work.

- [x] **Extra blank line on package comments**: `SaveAs()` preamble logic used `\n\n` between package doc comment and `package` keyword. Go requires the doc comment directly above `package` with no blank line. Fixed by emitting `\n` (no blank line) when the last preamble item is a package doc comment.

**Files**:

- `internal/provider/goast/sourcefile.go` — `SaveAs()` preamble logic

## Related Documents

- [Go Style Guidelines](../guides/go-style-guidelines.md)
- [Star Application Restructure](./star-application-restructure.md)
