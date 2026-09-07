---
title: "The README teaches retired writ verbs; it states the ruled interface and names the interim"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/183
status: in-progress
created: 2026-09-07
updated: 2026-09-07
---

# Plan: The README teaches retired writ verbs; it states the ruled interface and names the interim

## Summary

The "Deploying it" section #180 added on 2026-09-06 shows `writ repo add base <url>` and
`writ deploy common`. Both are retired: devlore-cli#791 (ruled 2026-09-04) replaces `repo add` and
`remove` with `set` and `unset`, and devlore-cli#850 (ruled 2026-09-06) makes `common` and the
repository-named projects implicit, so a bare `writ deploy` converges them. The section was written
from devlore-cli's `docs/guides/writ/repositories.md` and the binary's help, which still say the old
thing; those are devlore-cli#849's and #791's to fix. This README is the public advertisement, and it
states the ruled interface, with one sentence naming what today's binary still spells.

## Goals

1. The two commands are the ruled ones: `writ repo set base <url>`, `writ deploy`.
2. The prose says why nothing is named: `common` and the projects named for the configured layers
   are implicit.
3. One sentence names the interim and the two issues that end it, so a reader with today's binary
   knows what to type and the sentence retires itself.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| `README.md` "Deploying it" | retired verbs | `writ repo add`, `writ deploy common` |
| devlore-cli guide `repositories.md` | retired verbs | devlore-cli#849's pass |
| the shipped binary, build `155067ae` | retired verbs | `repo add`/`remove`; `deploy` refuses zero projects (devlore-cli#843) |

## Implementation Phases

### Phase 1: the section

- [x] Commands: `writ repo set base https://github.com/NobleFactor/noblefactor-ops.git`, `writ deploy`
- [x] The sentence before: `common` and the layer-named projects — here `noblefactor-ops` — are
      implicit; nothing is named unless it is extra
- [x] The sentence after: until devlore-cli#791 and devlore-cli#850 ship, the binary spells these
      `writ repo add` and `writ deploy common`
- [x] **Acceptance:** `git grep -n 'writ repo add\|writ deploy common' README.md` finds only the
      interim sentence; the section reads correctly to a stranger with either binary

**Files**: `README.md` — Modify

## Decisions

- **State the ruling, name the interim.** A README that shows commands the binary rejects misquotes
  reality one way; one that shows retired verbs misquotes it the other. One sentence carries both
  truths and deletes itself when the issues close.
- **Rulings over guides and help text.** Both lagged the rulings by days here. When a verb is in
  question, the issue that ruled it is the source; the guide and the binary are consumers of it.
- **The 2026-09-06 plan (#179) keeps its line.** It records what was written that day; a record is
  not swept.

## Related Documents

- Issue #183 — this chore; feature #158, epic #142; `Thread:WritOrigin`
- #179 / #180 — where the section was written
- devlore-cli#791 — `set`/`unset`; devlore-cli#850 — the implicit projects; devlore-cli#843 — the
  zero-argument refusal; devlore-cli#849 — the guide pass

## Open Questions

None.
