---
title: "The unit of work is a session, not a worktree, and features run in parallel"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/135
status: in-progress
created: 2026-09-02
updated: 2026-09-02
---

# Plan: The unit of work is a session

## Summary

`development-process.md` names one unit of work — the worktree — and therefore states "One open
worktree at a time" as a rule. Practice runs several features in parallel, each with its own
worktree. Three things nest, and the guide has room for only one of them. This plan gives it all
three, and corrects the rule that contradicts practice.

## Goals

1. Remove a rule the work does not follow, before it is cited as though it were.
2. Name the session, which is the unit the process actually turns on.
3. Name the feature, which is what parallel worktrees belong to.
4. Keep the legibility the old rule was protecting, by naming rather than by scarcity.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| "One open worktree at a time" | ❌ Contradicts practice | Approved status; work runs many features in parallel. |
| The session | ❌ Unnamed | The unit that actually bounds a piece of work. |
| The feature | ❌ Unnamed | What parallel worktrees belong to; declared by `Start-Claude --issue`. |
| `Discovering issues mid-work` | ⚠️ Needs a touch | "Current worktree" now implies a new session. |
| Section Status table | ⚠️ Stale | Row reads `One issue, one worktree`. |
| Reference implementation | ⚠️ Incomplete | Names the two git commands; not `Start-Claude`. |
| `docs/plans/doc/133-development-process.md` | ⚠️ Stale | `in-progress`; Requirement 2 asserts the old rule. |

## Requirements

### Requirement 1: Three units, correctly sized

| | What it is | How many at once |
| --- | --- | --- |
| **Feature** | A body of work spanning several linked issues | Many, in parallel |
| **Worktree** | One pull request, resolving one or more of that feature's issues, based on the default branch | One per pull request |
| **Session** | One issue, worked to completion | One per issue |

### Requirement 2: Retire "one open worktree at a time"

As many worktrees may be open as there are pull requests in flight. The rule's stated purpose was
legibility — "only one place to look" — and that is preserved by naming instead: `git-open-branch`
names each worktree `<repo>.<issue>-<name>`, so a directory listing is the whole picture.

What survives untouched: a branch requires an issue; a worktree may resolve more than one issue; no
pull request until every issue in it is resolved; a pull request resolves issues and nothing else.

### Requirement 3: The session, and that it is cheap to lose

A session targets a single issue and works it to completion. It is **disposable and easily
resumed** — closing one is normal, not a loss, because `Start-Claude` resumes the conversation for
that repository and branch. That is what makes a per-issue session affordable: were resuming
expensive, sessions would grow to fit the worktree and stop bounding anything.

### Requirement 4: The feature, declared at session start

`Start-Claude --issue <feature>` titles the session from the feature's issue, giving
`<repository> | <branch> | <feature>`. The branch already names the issue that opened the worktree,
so titling by the feature is what tells parallel sessions apart. With several features in flight the
session name is the only thing distinguishing one from another at a glance.

### Requirement 5: Consequential edits

- `Discovering issues mid-work`: resolving an issue "in the current worktree" means a new session for
  it, not more work inside the current one.
- Section Status: the row is restated for the three units.
- Reference implementation: `Start-Claude` joins `git-open-branch` and `git-close-branch`, because it
  is what makes the session layer real rather than aspirational.
- `docs/plans/doc/133-development-process.md`: Requirement 2 no longer asserts the retired rule.

## Implementation Phases

### Phase 1: The unit of work — complete

- [x] Replace the section with the three nested units
- [x] Retire "One open worktree at a time" and keep its legibility rationale, re-grounded on naming
- [x] State the session, disposable and easily resumed
- [x] State the feature and how a session declares it

### Phase 2: Consequential edits — complete

- [x] `Discovering issues mid-work`
- [x] Section Status row
- [x] Reference implementation
- [x] Correct Requirement 2 of plan 133

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/guides/development-process.md` | Modify | The revision |
| `docs/guides/release-process.md` | Modify | Its cross-reference asserted the retired rule |
| `docs/plans/doc/133-development-process.md` | Modify | Stop asserting the retired rule |
| `docs/plans/doc/135-unit-of-work-is-a-session.md` | Create | This plan |

## Out of Scope

- `release-process.md`'s branching and release mechanics, which are unaffected by which unit bounds a
  piece of work. Its one-line cross-reference to this guide did assert the retired rule, and is
  corrected here.
- `docs/guides/powershell-style-guidelines.md` is committed in this worktree under its own issue,
  #137, not under this one. It rides in the same pull request because it belongs beside these
  revisions, and it is listed here so this plan does not appear to claim it.

## Issue 135

[The unit of work is a session, not a worktree, and features run in
parallel](https://github.com/NobleFactor/noblefactor-ops/issues/135)

The whole plan serves this one issue.

## Related Documents

- Issue #135
- [136-issue-document-table.md](136-issue-document-table.md) — issue #136, resolved in this worktree
  because it revises the same document
- [137-powershell-style-guidelines.md](137-powershell-style-guidelines.md) — issue #137, resolved in this worktree
  because the guide belongs beside these revisions
- [138-squash-never-delete-branch.md](138-squash-never-delete-branch.md) — issue #138, resolved in this worktree
  because it revises a process document this pull request carries
- [133-development-process.md](133-development-process.md) — issue #133, the guide this revises
- `docs/guides/development-process.md` — the document under revision

## Open Questions

- [ ] Does a worktree belong to exactly one feature, or may it resolve issues from several? The
      revision assumes one, because a pull request that spans features is hard to review and
      impossible to revert per feature.
