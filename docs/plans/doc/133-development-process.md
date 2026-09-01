---
title: "The development process is practiced but unwritten"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/133
status: in-progress
created: 2026-09-01
updated: 2026-09-01
---

# Plan: Write down the development process

## Summary

The discipline we work by — one issue at a time, one open worktree, a plan committed before the work, and
every document updated on every commit — is practiced and unwritten. This plan adds
`docs/guides/development-process.md` as a new guide, indexes it in the README's standards table, and
records why it is a separate document rather than a section of the existing process guide.

## Goals

1. **Make the process official** — normative, citable, and the same across repositories.
2. **Keep it findable** — a guide an engineer consults daily is not buried at line 600 of a document
   about tagging and rollback.
3. **Keep it portable** — the rules stand without the tooling that makes them convenient.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| Branching, release, rollback, tagging | ✅ Documented | `docs/guides/release-process.md`, 683 lines |
| Worktree discipline | ❌ Unwritten | Zero occurrences of "worktree" in that guide |
| Issue discipline — logging, triage, blocking | ❌ Unwritten | Zero occurrences |
| Plan-before-work | ❌ Unwritten here | CLAUDE.md mandates it; no guide defines it |
| Documents-on-every-commit | ❌ Unwritten | |

`release-process.md`'s `## Development Phase` covers feature-branch mechanics and commit signing. Its
title, "Development and Release Process", appears to claim this ground, which is part of why the gap went
unnoticed.

## Requirements

### Requirement 1: A new guide, not a section

`docs/guides/development-process.md`. Rejected alternative: a section inside `release-process.md`. That
document is entirely branch and release mechanics, and discipline placed after 600 lines of rollback
procedure is not findable by the engineer who needs it daily. A separate guide also earns a row in the
README's standards table, which a subsection cannot.

### Requirement 2: The four rules, stated normatively

- **One open worktree at a time.** More than one issue may be resolved in it; no pull request until every
  issue in that worktree is resolved.
- **Issues are logged on discovery**, and where each is resolved is decided at that moment — current
  worktree (added to the current plan) or elsewhere (a plan created for it, cross-referenced from the
  current plan).
- **Blocking is defined**: we cannot proceed without addressing it, *and* it is either orthogonal to the
  work in hand or large enough to be its own piece of work. Blocking forces an explicit choice between a
  new worktree and delaying the pull request; proceeding as though it were absent is not an option.
- **The plan is committed before the work begins**, and every commit updates every document it touches.
  No deviations.

### Requirement 3: Plans are named after their issues

Settled while this work was open, and stated in the guide's `Plans` section:

```
docs/plans/<type>/<issue>-<name>.md
```

`<type>` comes from the alternation the organization ruleset enforces on every ref, and `<issue>-<name>`
is what the branch carries, so a plan's path below `docs/plans/` is its branch's name exactly. This plan
follows it — `docs/plans/doc/133-development-process.md` — and is the first in this repository to do so;
the twelve existing plans are named for their subjects and are left alone.

### Requirement 4: The scripts named once

`git-open-branch` and `git-close-branch` are named once, as the reference implementation. The rules are
normative without them, so a team that does not have them is held to the same process. They live outside
the NobleFactor repositories, which is the reason not to lean the guide on them.

### Requirement 5: Frontmatter conformance

The guide is a **catalogue** document under
[documentation-standards.md](../../documentation-standards.md): it declares `type: Process` and takes its
`status` from the catalogue vocabulary. This plan is a **working** document: no `type`, and `status` from
the working vocabulary. `.github/scripts/Test-Frontmatter.sh` gates both.

## Implementation Phases

### Phase 1: The guide (status: complete)

- [x] `docs/guides/development-process.md` written — the unit of work, discovering issues mid-work,
      blocking issues, plans, documents on every commit, reference implementation
- [x] Frontmatter in the catalogue family, matching `release-process.md`'s shape
- [x] Cross-references to `release-process.md`, `TEMPLATE.md`, `documentation-standards.md`,
      `github-branch-protection.md`, `pr-script-template.md`

### Phase 2: Indexing and cross-linking (status: complete)

- [x] README's "The standards CLAUDE.md points at" table gains a row for the new guide
- [x] `release-process.md` gains a pointer to it, so a reader who starts there is sent to the discipline
      it does not cover

**Files**:

- `docs/guides/development-process.md` - Create
- `README.md` - Modify
- `docs/guides/release-process.md` - Modify

## Test Plan

No code. The gate that applies is the frontmatter check, and the claims about the existing guide are
verifiable by search.

| # | What it proves | Level | Fails when |
| --- | --- | --- | --- |
| 1 | The new guide satisfies the frontmatter standard | CI | `Test-Frontmatter.sh` rejects a missing or out-of-vocabulary `type`/`status` |
| 2 | The gap this plan claims is real | search | `grep -i worktree docs/guides/release-process.md` returns a match, falsifying the premise |

**Not covered:** whether the process as written is the process as practiced. That is settled by using it,
and the first work to follow it is the four-thread setup chore in `devlore-cli`.

## Migration Path

None. The guide records what is already being done; nothing changes for work already in flight.

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/guides/development-process.md` | Create | The guide |
| `README.md` | Modify | Index it in the standards table |
| `docs/guides/release-process.md` | Modify | Point at it from the adjacent process guide |

## Related Documents

- [development-process.md](../guides/development-process.md) — the guide this plan creates
- [release-process.md](../guides/release-process.md) — the sibling it defers to for branching and releases
- [documentation-standards.md](../documentation-standards.md) — the frontmatter families
- Issue [#133](https://github.com/NobleFactor/noblefactor-ops/issues/133)

## Open Questions

- [ ] Should `CLAUDE.md` — the private global instructions — be reduced to a pointer at this guide for the
      rules it now duplicates? Doing so removes a second place for the process to drift, but that file is
      outside any repository and cannot be reviewed here.
