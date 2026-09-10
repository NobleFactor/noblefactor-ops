---
title: "A plan reaches complete in its pull request's last commit"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/199
status: complete
created: 2026-09-10
updated: 2026-09-10
---

# Plan: A plan reaches complete in its pull request's last commit

## Summary

A merged file cannot be edited by its own merge, so a plan whose closure was understood to come "at merge"
finished every pull request at `chartered` with one open box, and closing it honestly took a second pull
request. Ruled 2026-09-10: the last commit of a pull request sets the plan's status to `complete` when every
other box is ticked. `development-process.md` gains the rule beside "phase status is updated the moment a
phase completes", and the status ladder a plan moves through is written down in the same place, since the
template still names a status nobody uses.

## Issue 199

A chore under [#157](https://github.com/NobleFactor/noblefactor-ops/issues/157), the PR tooling. The case
that raised it: devlore-cli#814's plan merged 2026-09-10 as devlore-cli#879 at `chartered` with its closure
box open, by the letter of a ruling made during its review. This plan is the first closed under the rule: its
own second commit ticks its boxes and sets `complete`.

## Goals

- [x] `development-process.md` states the rule: the last commit of a pull request sets `complete` when every
      other box is ticked; the merge is not a state the plan records.
- [x] The same paragraph states the ladder: `draft` until reviewed, `chartered` from the review, `complete`
      from that last commit, `abandoned` when the work is dropped.
- [x] `docs/plans/TEMPLATE.md` names the same four statuses; `in-progress` goes.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| `development-process.md` §Documents on every commit | ⚠️ | phases update the moment they complete; nothing says when the plan itself completes |
| `chartered` | ⚠️ used, undefined | appears in plans only; no process document names it |
| `docs/plans/TEMPLATE.md` | ❌ stale | `status: draft \| in-progress \| complete \| abandoned` |

## Requirements

### Requirement 1: The rule, where phase status is ruled

A fourth bullet under "In particular", and one paragraph after the list: the last commit of a pull request
sets the plan's status to `complete` when every other box is ticked; the closure box is that commit's; a
plan whose boxes are not all ticked at the last commit stays `chartered`, and the pull request says why.
The paragraph names the case, devlore-cli#814, in one sentence.

### Requirement 2: The ladder, stated once

The same paragraph names the four statuses and when each holds. The template's frontmatter comment lists
the same four, so a plan copied from it cannot start with a status the process does not know.

## Implementation Phases

### Phase 1: The plan

- [x] This document, the branch's first commit.

### Phase 2: The rule and the ladder

- [x] `development-process.md`: the bullet, the paragraph, `updated: 2026-09-10`.
- [x] `docs/plans/TEMPLATE.md`: the status line.
- [x] **Acceptance:** the frontmatter gate and codespell pass; this plan's second commit ticks every box above
      and sets `complete`, which is the rule applied to itself.

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/plans/chore/199-plan-complete-at-last-commit.md` | Create | this plan, the first commit |
| `docs/guides/development-process.md` | Modify | the rule and the ladder |
| `docs/plans/TEMPLATE.md` | Modify | the status line |

## Related Documents

- [#199](https://github.com/NobleFactor/noblefactor-ops/issues/199) -- this plan; feature #157, epic #142
- [devlore-cli#814](https://github.com/NobleFactor/devlore-cli/issues/814) -- the case; its plan is flipped in a follow-up under the rule
- [#200](https://github.com/NobleFactor/noblefactor-ops/issues/200) -- found opening this branch: the opener types a chore as `feature/`
