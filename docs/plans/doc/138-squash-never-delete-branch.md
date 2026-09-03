---
title: "The PR script merges with --squash and never --delete-branch, and the template does not say so"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/138
status: complete
created: 2026-09-03
updated: 2026-09-03
---

# Plan: Always --squash, never --delete-branch

## Summary

A PR script merged personal#161 with `gh pr merge --squash --delete-branch`. The merge landed; the
local delete then failed because a linked worktree held the branch; `gh` exited non-zero; and
`set -euo pipefail` ended the script after the merge and before any cleanup. The template's merge
line was already correct, but nothing forbade adding to it. This writes the rule into the template
in the two places a reader will meet it: a comment on the merge line, and a Rule.

## Goals

1. Make the merge command's shape a stated rule, not a convention inferred from one example.
2. Say *why*, so the next reader does not re-add the flag as a convenience.

## Requirements

### Requirement 1: The merge line explains itself

A comment above `gh pr merge "${pr_number}" --squash --admin` states that `--squash` is always
present and `--delete-branch` never is, and why: the flag deletes the local branch first, which
fails under a linked worktree, and the failure lands after the merge and before cleanup.

### Requirement 2: A Rule

Rule 8 states both halves, and names the order that survives a failure — remote, then worktree,
then local — which is `git close-branch`'s order and the opposite of `--delete-branch`'s.

## Implementation Phases

### Phase 1: The template — complete

- [x] Comment on the merge line
- [x] Rule 8

## Issue 138

[The PR script merges with --squash and never --delete-branch, and the template does not say
so](https://github.com/NobleFactor/noblefactor-ops/issues/138)

The whole plan serves this one issue. It is resolved in the #135 worktree because it revises a
process document that pull request already carries.

## Related Documents

- Issue #138
- [135-unit-of-work-is-a-session.md](135-unit-of-work-is-a-session.md) — the plan sharing this worktree
- `docs/guides/pr-script-template.md` — the document under revision
- David-Noble-at-work/personal#161 — the PR whose script exposed this
