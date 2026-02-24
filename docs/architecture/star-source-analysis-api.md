```markdown
---
title: "Star Source Analysis API"
description: "Architecture for capturing, indexing, summarizing, and analyzing source sets for CI reporting (Starlark-focused)"
status: draft
created: 2026-02-24
updated: 2026-02-24
---

# Star Source Analysis API

This document proposes a cohesive Starlark-facing API for analyzing source sets in CI and developer workflows, with an initial focus on **Starlark** sources.

## Goals

- Provide a clean, object-oriented workflow in Starlark:
  - capture a source set (glob)
  - produce an index (deep dive)
  - produce quick stats (fast summary)
  - produce an analysis report (complexity + stats)
- Work well alongside other language analyzers (Go, shell), producing similarly shaped artifacts.
- Avoid needing explicit resource cleanup (no `with`, no `try/finally` in Starlark).
- Keep naming consistent and readable: use `index` (not `ast`, not `idx`).

## Non-goals

- Expose a full node-by-node compiler AST to Starlark scripts.
- Require deterministic teardown semantics for in-memory results.

## Overview (Starlark)
```
python
sources = starlark.capture("src/**/*.star")

# Deep Dive: returns an index of parsed structure across files
index = sources.index()

# Quick Stats: lightweight counters for CI summaries
stats = sources.stats()

# Full Report: complexity + stats (and optional index embedding)
report = sources.analyze()
```
## API: `starlark`

### `starlark.capture(pattern, *, gitignore=True, include_bzl=True) -> StarlarkSources`

Captures a set of Starlark source files for repeated analysis.

Arguments:

- `pattern` (string, required)
  - Glob pattern or a single path.
  - Intended to match `.star` and optionally `.bzl` files.
- `gitignore` (bool, default `True`)
  - If `True`, matched files are filtered using `.gitignore` rules.
- `include_bzl` (bool, default `True`)
  - If `True`, include `.bzl` files as Starlark sources as well.

Semantics:

- `capture()` should hold only:
  - the capture configuration, and
  - the resolved list of file paths (or the pattern + lazy resolution, if desired).
- `capture()` must not hold external resources (open file descriptors, watchers, temp dirs).
  - This avoids the need for `sources.close()` in Starlark.

Returns:

- `StarlarkSources`: an opaque handle for the captured source set.

## API: `StarlarkSources`

### `sources.index(*, with_docstrings=True, with_globals=True) -> StarlarkIndex`

Builds a deep structural index across all captured sources.

Arguments:

- `with_docstrings` (bool, default `True`)
- `with_globals` (bool, default `True`)

Returns:

- `StarlarkIndex` containing per-file indexed structures and totals.

Notes:

- This is “AST-derived structure” (functions, loads, globals), not a full syntax tree.
- The index may also include basic counters (LOC/SLOC) since those are cheap and useful.

### `sources.stats(*, bytes=True, loc=True) -> StarlarkStats`

Computes quick, CI-friendly statistics.

Arguments:

- `bytes` (bool, default `True`)
  - Include byte counts (per-file and totals).
- `loc` (bool, default `True`)
  - Include LOC/SLOC/comments/blanks.

Returns:

- `StarlarkStats`

Notes:

- `stats()` is designed to be fast and stable.
- It should not require parsing if the caller only wants file size + file count.

### `sources.analyze(*, hotspots=True, cyclomatic_threshold=10, cognitive_threshold=15, with_index=False) -> StarlarkAnalysisReport`

Produces a full analysis report for CI.

Arguments:

- `hotspots` (bool, default `True`)
  - If `True`, include a pre-filtered list of functions exceeding thresholds.
- `cyclomatic_threshold` (int, default `10`)
- `cognitive_threshold` (int, default `15`)
- `with_index` (bool, default `False`)
  - If `True`, embed the `StarlarkIndex` into the report.

Returns:

- `StarlarkAnalysisReport`

Notes:

- Default behavior should avoid embedding the index to keep report size manageable.
- `analyze()` always includes `stats` so CI can consume one artifact per language.

## Data Structures

### `StarlarkIndex`

Suggested shape:

- `files: list[StarlarkIndexedFile]`
- `totals: StarlarkIndexTotals`

#### `StarlarkIndexedFile`

- `path: string`
- `functions: list[StarlarkFunctionDecl]`
- `loads: list[StarlarkLoadDecl]`
- `globals: list[StarlarkGlobalAssign]` (optional based on `with_globals`)
- `loc: int`
- `sloc: int`
- `comments: int`
- `blanks: int`

#### `StarlarkIndexTotals`

- `file_count: int`
- `functions: int`
- `loads: int`
- `globals: int`
- `loc: int`
- `sloc: int`
- `comments: int`
- `blanks: int`

### `StarlarkStats`

Suggested shape:

- `files: list[StarlarkFileStats]`
- `totals: StarlarkStatsTotals`

#### `StarlarkFileStats`

- `path: string`
- `bytes: int` (if `bytes=True`)
- `loc: int` (if `loc=True`)
- `sloc: int` (if `loc=True`)
- `comments: int` (if `loc=True`)
- `blanks: int` (if `loc=True`)

#### `StarlarkStatsTotals`

- `file_count: int`
- `total_bytes: int` (if `bytes=True`)
- `total_loc: int` (if `loc=True`)
- `total_sloc: int` (if `loc=True`)
- `total_comments: int` (if `loc=True`)
- `total_blanks: int` (if `loc=True`)

### `StarlarkAnalysisReport`

Suggested shape:

- `stats: StarlarkStats`
- `complexity: StarlarkComplexityReport`
- `hotspots: list[StarlarkHotspot]` (if `hotspots=True`)
- `index: StarlarkIndex | None` (if `with_index=True`)

#### `StarlarkComplexityReport`

- `files: list[StarlarkFileComplexity]`
- `totals: StarlarkComplexityTotals`

`StarlarkFileComplexity` includes per-function metrics (cyclomatic, cognitive, nesting depth, LOC, params).

#### `StarlarkHotspot`

A compact object suitable for CI output:

- `file: string` (basename or relative path; decide consistently)
- `path: string` (optional full/relative path)
- `name: string`
- `line: int`
- `cyclomatic: int`
- `cognitive: int`
- `loc: int`

## Naming & CI considerations

This API is designed to coexist with other language analyzers producing parallel artifacts:

- `go.capture(...).index() -> GoIndex`
- `go.capture(...).stats() -> GoStats`
- `go.capture(...).analyze() -> GoAnalysisReport`

- `shell.capture(...).index() -> ShellIndex`
- `shell.capture(...).stats() -> ShellStats`
- `shell.capture(...).analyze() -> ShellAnalysisReport`

Type names are language-prefixed to avoid ambiguity in multi-language CI pipelines.

## Resource lifecycle

Starlark does not provide `with`/context managers or `try/finally`. Therefore:

- `StarlarkSources` should not require an explicit `close()` method.
- If internal caching is introduced later, prefer:
  - `sources.clear_cache()` (drops cached derived results; optional enhancement)
- Avoid external resources in analysis objects.

## Open Questions

1. Should `capture()` resolve the glob eagerly or lazily?
2. Should `.bzl` inclusion be implicit (default) or explicit?
3. Should paths in outputs be absolute, project-relative, or caller-provided?
4. Should `index()` include counters by default, or keep counters exclusively in `stats()`?
```

