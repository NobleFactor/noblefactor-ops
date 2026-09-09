---
title: "The placement rule names its exemptions, so common* stops being both forbidden and occupied"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/195
status: complete
created: 2026-09-09
updated: 2026-09-09
---

# Plan: the placement rule names its exemptions

## Summary

`development-process.md` says no bash script lives under `common*`, and six lines later says
`Declare-BashScript` and the `git-*` commands are the base's `common`. Both sentences are in the same
section. A reader following the rule as written sends the trio to `Home/noblefactor-ops`, and #146 is
exactly that reader: its body named `Home/common` correctly, and this session argued it into being wrong
by quoting the blanket sentence.

The rule bars a script that depends on something **outside** `common`, because `common` deploys
unconditionally and must depend on nothing outside itself. The trio's only such dependency is
`Declare-BashScript`, which is in `common` too, so nothing points out. They are there because Git for
Windows needs them wherever git is used, whatever layers are selected — which is what `common` is for.

## Goals

1. **The rule is stated for the case it governs**: bash scripts in *consuming* repositories.
2. **Its exemptions are named where the rule is stated**, not left to a consequence further down.
3. **The reasoning survives**, so the next reader can tell an exemption from an exception: what `common`
   forbids is a dependency pointing out of it.

## Current State

| Location | Text | Problem |
| --- | --- | --- |
| `docs/guides/development-process.md:243-247` | "every bash script depends on the base, and no bash script lives under `common*`. **A git hook is exempt**" | Blanket; names one exemption of three |
| `docs/guides/development-process.md:252` | "**`Declare-BashScript` and the `git-*` commands are the base's `common`.**" | Correct, and contradicts the above |
| `docs/plans/chore/147-declare-bashscript.md` | the same blanket phrasing | A completed plan: a record of what was ruled when, not normative. Not edited |

## Requirements

The third bullet of "Where a script lives" states the rule for consuming repositories and names both
exemptions in the base:

- **`Declare-BashScript` and the `git-*` commands**, in this repository's `common`. Git for Windows brings
  bash, so the git-* commands must be present wherever git is used; the helper they source sits beside
  them in the same `~/.local/bin`, so the dependency stays inside `common`.
- **A git hook**, ruled 2026-09-08: git runs it from the hooks path, where there is no sibling to source.

The consequence bullet at line 252 then reads as elaboration, and is trimmed of what the rule now says.

## Implementation Phases

### Phase 1: the plan

- [x] This document, committed as the branch's first commit

### Phase 2: the rule

- [x] The third bullet of "Where a script lives" rewritten as above, with both exemptions named as
      sub-bullets and the reason stated once: what `common` forbids is a dependency pointing out of it
- [x] The consequence bullet trimmed to "In a consuming layer, the selector says what the script needs",
      which is what it uniquely said; the duplicated claim is gone
- [x] Swept `README.md` and `docs/` for the blanket phrasing: the only other occurrence is
      `docs/plans/chore/147-declare-bashscript.md`, a completed plan and therefore a record, not edited

### Phase 3: the downstream copy

- [ ] `personal/Home/common/.claude/CLAUDE.md` carries the same sentence and is the source of
      `~/.claude/CLAUDE.md`. It is that repository's file, so it is filed and fixed there, and this plan
      records the dependency rather than reaching across the boundary

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/plans/chore/195-common-carve-out.md` | Create | this plan, the branch's first commit |
| `docs/guides/development-process.md` | Modify | the rule states its exemptions |

## Decisions

- **The exemptions are named where the rule is, not only in the consequences.** A rule whose exceptions
  live further down the page is a rule that will be quoted without them. It already was.
- **Completed plans are not edited.** `147-declare-bashscript.md` records what was ruled on 2026-09-07 and
  2026-09-08. Rewriting it would falsify the record; the guide is where the current rule lives.
- **The `personal` copy is filed separately.** One repository at a time, and that file belongs to that
  repository.

## Related Documents

- Issue #195; #146 (blocked by this — the move cannot run from a document that says its destination is
  wrong); #147 (which established the placement rule)

## Open Questions

None.
