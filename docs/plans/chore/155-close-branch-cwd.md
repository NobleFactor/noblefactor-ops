---
title: "The PR script template steps back to the main clone after close-branch"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/155
status: complete
created: 2026-09-05
updated: 2026-09-05
---

# Plan: The PR script template steps back to the main clone after close-branch

## Summary

The template's closing sequence — `git close-branch` then `git status --short` — exits 128 whenever
the script runs from a worktree, because close-branch removes the worktree the script is standing in.
Under `git-open-branch` that is the normal case. Observed on devlore-cli#811: merge, remote delete and
worktree removal all succeeded, then the last line failed from a deleted directory. Cosmetic, and
corrosive: a gated script that exits non-zero after success teaches the reader to distrust exit codes.

## Goals

1. **The template's closing sequence survives its own cleanup** — `cd` to the main clone before
   anything runs after `git close-branch`.
2. **The rule is stated**, so the next script author does not rediscover it.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| Template closing block | ❌ Wrong | `git close-branch` is the last line; anything after it fails |
| Rules list, item 6 | ❌ Incomplete | says close-branch handles worktrees; not that it removes the one you are in |
| `go-noblefactor-ops` for #148 | ✅ Already fixed | stepped back before `git status`; exited clean |

## Implementation Phases

### Phase 1: The template — complete

- [x] Add `cd ~/Workspace/NobleFactor/<repo>` and `git status --short` after `git close-branch`, with
      the comment saying why
- [x] Extend rule 6 with the worktree-removal consequence and the #811 observation

**Files**: `docs/guides/pr-script-template.md` — Modify

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/guides/pr-script-template.md` | Modify | The closing sequence and rule 6 |
| `docs/plans/chore/155-close-branch-cwd.md` | Create | This plan |
| `docs/plans/chore/148-document-threads.md` | Modify | Status to complete; Phase 3 done under #154, #155 and the #140 rewrite |

## Related Documents

- Issue #155 — this chore
- Issue #157 — PR tooling, its feature; #145's `git-submit-branch` supersedes the template entirely
- `NobleFactor/devlore-cli#811` — where the failure was observed
