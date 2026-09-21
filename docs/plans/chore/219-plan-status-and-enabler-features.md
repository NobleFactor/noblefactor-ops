---
title: "Plan statuses are five words with a state diagram; the process epic's features are enablers"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/219
status: complete
created: 2026-09-21
updated: 2026-09-21
---

# Plan: Plan statuses, and the process epic's features

Two process-documentation chores under feature #156, ruled by the owner on 2026-09-21 and settled in
conversation before this branch opened.

## Issue 219

A plan's statuses are `draft`, `approved`, `active`, `complete`, `abandoned`. `chartered`, introduced
by #199, was not the owner's word; `in-progress` says what `active` says. The vocabulary lived in four
places and disagreed: the development process and the template said `chartered`, the documentation
standards said `in-progress`, and the frontmatter gate accepted seven words including neither
`chartered` nor the ruled five exactly.

- [x] `development-process.md`: the five statuses as a table, and a mermaid state diagram of the
      transitions -- ruled: "i want the diagram in our development process guidelines"
- [x] `plans/TEMPLATE.md` lists the five; `documentation-standards.md` points at it
- [x] `Test-Frontmatter.sh` accepts exactly the five, with a comment naming where they are defined
- [x] Seven plans carrying `in-progress` read `active` -- mechanical, since the two words mean the same
- [x] The gate passes: 53 checked, 0 errors

## Issue 222

`issue-standards.md` said a chore files "exactly as a task does: under a feature", which read as
though process work had product features. Settled: under a product epic a feature is product
functionality; under `Epic:Ops:Process` it is an enabler -- a body of process and tooling work, in the
literature's term. The leaves are task and bug on the product axis, chore on the process axis. The
scheme has no story.

- [x] § What a chore is: the enabler reading, the two-axis table, and "no story"
- [x] § The process epic: its features are enablers; a process or tooling problem is logged as a chore
      under the feature it improves

## Out of Scope

- The five kinds, the labels and the audit: unchanged.
- `docs/plans/chore/199-plan-complete-at-last-commit.md` keeps `chartered` in its text: it is the
  record of that change and says so.
