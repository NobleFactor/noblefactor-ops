---
title: "Every issue links its plan and design documents, and the process does not say so"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/136
status: complete
created: 2026-09-03
updated: 2026-09-03
---

# Plan: The issue's document table

## Summary

Every issue ends with a table linking its plan and the design documents it bears on. This is
practised on six issues across three repositories and written nowhere. `development-process.md`
attaches a plan to a task rather than an issue, requires cross-references only between plans, and
says nothing about the issue body. This plan adds the rule under **Plans**, in the worktree that is
already revising that document.

## Goals

1. Write down a rule that is already being followed, before it drifts.
2. Settle where the plan link points, so the link survives the branch's deletion.
3. Settle how a shared plan is addressed, so the link lands on the right section.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| The table, in practice | ✅ Six issues carry it | personal#160, #161; ops#135; devlore#782, #784, #785 |
| The table, in the guide | ❌ Absent | Nothing addresses the issue body |
| Plan-to-design cross-reference | ❌ Absent | L96 cross-references plans to plans only |
| `## Issue NNN` headings | ✅ Four plans carry them | Unwritten convention |
| Link target | ✅ Default branch, in practice | Unwritten; the alternative dies with the branch |

## Requirements

### Requirement 1: The table

Every issue ends with `## Plan and design documents`: two columns, `Kind` and `Document`, one row
for the plan and one per design document, each a link to a committed file. Design documents are not
required — an issue that bears on none has a plan row alone — but where they exist they are linked,
not named.

### Requirement 2: Links go to the default branch

`main` or `develop`, whichever the repository uses. The link is dead until the branch merges and
correct forever after. The alternative — linking the feature branch — is correct until
`git-close-branch` deletes it, which is the moment the issue closes and becomes most likely to be
read.

### Requirement 3: A plan is addressed by section

A plan carries an `## Issue NNN` heading for each issue it serves: the number alone, no `#`, no
title. GitHub derives the anchor from the heading text, so a heading that carries the title breaks
every link to it when the title is reworded; the number alone gives `#issue-nnn`, which cannot
change. Two issues may share a plan, and the table links the section rather than the file.

### Requirement 4: Timing

The guide already says the plan is written when the branch opens (L147), and that stands. An issue
whose branch has not opened therefore has no plan yet, and its plan row says so rather than being
omitted. The row is completed when the plan is committed. This reconciles "every issue has a plan"
with "the plan is committed before the work begins": every issue carries the table from the start,
and the plan arrives at the moment the process already places it.

## Implementation Phases

### Phase 1: The guide — complete

- [x] Add the rule under **Plans** in `docs/guides/development-process.md`
- [x] Revise the **Plans** row of the Section Status table
- [x] Add the table to this issue and to #135, so the guide's own issues obey it

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/guides/development-process.md` | Modify | The rule, under Plans |
| `docs/plans/doc/136-issue-document-table.md` | Create | This plan |
| `docs/plans/doc/135-unit-of-work-is-a-session.md` | Modify | Cross-reference, since both resolve in one worktree |

## Issue 136

[Every issue links its plan and design documents, and the process does not say
so](https://github.com/NobleFactor/noblefactor-ops/issues/136)

The whole plan serves this one issue. It is resolved in the #135 worktree because both revise the
same document.

## Related Documents

- Issue #136
- [135-unit-of-work-is-a-session.md](135-unit-of-work-is-a-session.md) — the plan sharing this worktree
- `docs/guides/development-process.md` — the document under revision
