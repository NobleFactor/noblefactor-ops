---
title: "buildifier gates the repository's Starlark"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/164
status: complete
created: 2026-09-05
updated: 2026-09-05
---

# Plan: buildifier gates the repository's Starlark

## Summary

Since #161 this repository holds three `.star` files and its CI reads none of them. Ruled
2026-09-05: gate them with `buildifier` now rather than wait for the resolution checker
(devlore-cli#721), which is a Go built-in and larger. One CI step, pinned to a release and its
digest.

## Goals

1. **Every tracked `.star` file passes `buildifier` on every push and pull request.**
2. **The pin cannot drift** — a release tag and the asset's SHA-256, both in the workflow.
3. **The house style is enforced, not Google's** — one-line docstrings; the two docstring-completeness
   warnings off, everything else on.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| `.star` files in the tree | 3 | `com.noblefactor.ops.GitHub/commands/` |
| CI reading them | ❌ None | frontmatter and codespell only |
| Local gate | Stopgap | the #161 PR script's pre-flight |
| The real linter | devlore-cli#721 | a Go built-in beside the code model; `star lint starlark` |

## Implementation Phases

### Phase 1: The step — complete

- [x] `ci.yaml` gains a `Starlark` step after Spelling: download `v8.5.1` linux-amd64, verify
      `sha256sum --check --strict`, run over `git ls-files '*.star'`
- [x] The header comment names Starlark among what the gate asserts
- [x] `pr-script-template.md`'s CI table carries the command for this repository
- [x] The pinned asset's digest verified against the real download
- [x] A deliberately malformed file fails the same command locally

**Files**: `.github/workflows/ci.yaml`, `docs/guides/pr-script-template.md` — Modify

## Related Documents

- Issue #164 — this chore
- Issue #156 — its feature, issue standards and the documents that state them
- `NobleFactor/devlore-cli#721` — the resolution checker; `buildifier` is the interim
- `NobleFactor/devlore-cli#340` — Phase 2 of the unified gate, `star lint starlark`
