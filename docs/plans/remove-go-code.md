---
title: "Remove the Go code and re-base the quality gate on documents"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/XX
status: draft
created: 2026-08-27
updated: 2026-08-27
---

# Plan: Remove the Go code and re-base the quality gate on documents

## Summary

`star` and its extensions moved to devlore-cli in March 2026. What stayed behind kept
compiling, so nothing announced that it had stopped mattering. This repository builds nothing
that anyone consumes: its Go last changed on 2026-06-13, and every commit since has been
documentation. Meanwhile the leftover `cmd/star` was still being built — by devlore-registry's
CI, for five months, producing the wrong binary and the failure tracked as devlore-cli#695.

This removes all 143 Go and Starlark files and re-bases `quality-gate` on what the repository
actually holds: documents.

## Current State

| Path | Files | Notes |
| --- | --- | --- |
| `internal/` | 104 | Last touched 2026-06-13 (#124) |
| `star/` | 33 | The same 16 `com.noblefactor.star.*` extensions devlore-cli embeds — **diverged** |
| `cmd/` | 1 | `cmd/star/main.go`, last touched 2026-03-22 (#123) |
| `go.mod`, `go.sum`, `.golangci.yaml` | 3 | |
| `Makefile` | 1 | Entirely star/Go: `star`, `build`, `generate`, `test`, `build-extensions` |
| `scripts/check-codegen.sh` | 1 | Go codegen verification |
| **Total to remove** | **143** | |

Surviving: `docs/` (30), `scripts/` (3 infra setup), `.github/` (2), `README.md`, `LICENSE`,
two root plan documents.

## The constraint that shapes this

`.github/workflows/ci.yaml` declares exactly one job, and its name is **`quality-gate`** — the
context org ruleset 12426847 requires on every repository. Deleting the workflow would leave a
required check that never arrives, which blocks every future pull request rather than none.

So the gate is **replaced, not removed**, in the same change. The replacement is the documents
gate already proven in the `noblefactor` repository: frontmatter validation, pre-commit hooks,
and codespell.

## Requirements

### Removal is one commit, not a trickle

143 files across three trees plus build configuration. Split across PRs, there is a window where
the repository has code that nothing builds and a gate that checks nothing.

### star/ goes, despite being Starlark rather than Go

Outside the literal instruction, but the same 16 extensions live in devlore-cli's
`cmd/star/extensions/` in a newer form — the copies here still carry a `receivers:` block that
devlore-cli has since dropped. A diverged second copy is how the next person builds the wrong
thing, which is precisely what happened with `cmd/star`.

### The gate keeps its name

The replacement job must be named `quality-gate` exactly. A rename is a separate decision
requiring a ruleset change, and coupling the two would freeze the repository.

## Implementation Phases

### Phase 1: Remove

- [ ] `git rm -r internal cmd star`
- [ ] `git rm go.mod go.sum .golangci.yaml Makefile scripts/check-codegen.sh`
- [ ] Confirm no surviving file references the removed trees in a load-bearing way

**Files**: 143 — Delete

### Phase 2: Re-base the gate

- [ ] Rewrite `.github/workflows/ci.yaml`: job stays `quality-gate`, steps become
      frontmatter / pre-commit / codespell
- [ ] Port `.github/scripts/Test-Frontmatter.sh`, `.github/frontmatter-exempt`, and
      `.github/codespell-ignore` from the `noblefactor` repository
- [ ] Drop the app-token and `GOPRIVATE` steps — they exist to fetch private Go modules
- [ ] Verify the gate is green on the branch **before** merging; a red gate on a repository
      whose only required check is that gate cannot be merged past without an override

**Files**: `.github/workflows/ci.yaml` — Modify; `.github/scripts/`, `.github/frontmatter-exempt`,
`.github/codespell-ignore` — Create

### Phase 3: Documentation truth

- [ ] `README.md` still describes a build; correct it
- [ ] Eleven documents under `docs/` reference `star/extensions/`, `internal/provider`, or
      `internal/extension`. They are historical records of work that happened, so their paths
      become dangling by design — add a note rather than rewriting history

**Files**: `README.md` — Modify

## What this does not do

It does not fix devlore-registry. Both `validate.yaml` and `update-indexes.yaml` check this
repository out and run `go build ./cmd/star`; after this they fail at checkout rather than at
runtime. Both already fail, so nothing regresses — but the registry's fix lives in devlore-cli's
`docs/plans/goreleaser-ship-star.md` and must land for the freeze to lift.

## Open Questions

- [ ] Do the three surviving `scripts/setup-*.sh` still work, or are they also stale?
- [ ] `PLAN-starlark-extensions.md` and `draft-star-lint-deadcode.md` sit at the root and
      describe the removed code — archive under `docs/`, or delete?
- [ ] Should `docs/` gain a note recording that star moved to devlore-cli, so the next reader
      does not have to reconstruct it from git history?
