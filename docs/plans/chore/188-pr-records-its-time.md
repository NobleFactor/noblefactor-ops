---
title: "Every pull request records its time"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/188
status: complete
created: 2026-09-08
updated: 2026-09-08
---

# Plan: Every pull request records its time

## Summary

A pull request should say how long it took, in its own body, computed the same way every time. Two have carried a
`## Time` section so far -- devlore-cli#867 and #869 -- because a coding agent put the computation in those merge
scripts by hand. Nothing in the process says to, so the next script written without that memory omits it, and a
series computed two ways is not a series. This plan writes the practice into the two guides that govern a pull
request, so it holds because the process says so.

## Issue 188

The practice was asked for on 2026-09-08: "let's give that a try", then "If this is going to be useful, it must be
recorded with every PR using the same algorithm", then "that belongs in the process documents. I really want that
there." This plan is the documents half. Closed by this plan's pull request.

## Goals

- [x] The PR-script template's body skeleton carries a `## Time` section.
- [x] Its merge step computes the section and refuses to finish without it.
- [x] A rule states the practice, its three markers, and that effort is not recorded in the pull request.
- [x] `development-process.md` names it beside the end-of-PR report.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| The practice | ⚠️ two PRs | devlore-cli#867 (11.4 h) and #869 (2.2 h), by hand in each merge script |
| `pr-script-template.md` | ❌ silent | ten rules, none of them this; the body skeleton has Summary and Test plan only |
| `development-process.md` | ❌ silent | says every pull request ends with the report; says nothing about its time |
| `star gh pr span` | ❌ not built | the command that would own the algorithm; this plan carries the algorithm verbatim until it exists |

## Requirements

### Requirement 1: Three markers, one algorithm

A pull request records three moments, from GitHub's own timestamps:

| Marker | Source |
| --- | --- |
| `opened` | the branch's first commit -- `commits[0].committedDate` |
| `submitted` | the pull request's `createdAt` |
| `merged` | its `mergedAt` |

and the elapsed hours between them, to one decimal. What it measures is how long the work was open, review gaps and
nights included. It is not an effort measure and must not be read as one.

### Requirement 2: The merge step writes it, and refuses without it

`mergedAt` exists only after the merge, so the section is appended immediately **after** `gh pr merge` and its
presence verified; a missing section fails the script loudly rather than passing silently. The template carries the
`gh`/`jq` computation verbatim. When `star gh pr span` exists, the template calls it and the verbatim block goes.

### Requirement 3: Effort stays out of the pull request

Active working time is not in GitHub. Where a coding agent estimates it from its own session transcript, that
estimate goes in the agent's end-of-PR message, labeled an estimate, and never into the pull request as fact. The
guides say so, so that the two measures are never confused.

### Requirement 4: The practice is a rule, not a memory

Rule 11 of the PR-script template states it, and `development-process.md` names it in the sentence that already says
every pull request ends with the report -- the two things a pull request leaves behind, in one place.

## Design

**Why the template and not a hook.** A hook cannot see the PR body, and the merge step already exists in every
script the template produces. Putting the computation where the merge happens is what makes it unavoidable.

**Why verbatim now and a command later.** The command is the right home (issue #188 specifies `star gh pr span`), but
a rule that waits for a tool is a rule nobody follows. The template carries the block, and the command replaces it.

**Why "time" and not a coined name.** The practice needs no jargon: a pull request records its time.

## Implementation Phases

### Phase 1: The template -- COMPLETE (2026-09-08)

- [x] The PR body skeleton gains `## Time` with a placeholder line saying the merge step fills it.
- [x] The merge step gains the computation, the append, and the verification, after `gh pr merge`.
- [x] Rule 11 states the practice, the three markers, and the effort exclusion.

### Phase 2: The process document -- COMPLETE (2026-09-08)

- [x] `development-process.md` names it beside the end-of-PR report.

**Files:** `docs/guides/pr-script-template.md`, `docs/guides/development-process.md`.

## Test Plan

Documentation; the repository's own gate is the check.

| # | What it proves | Level | Fails when |
| --- | --- | --- | --- |
| 1 | Frontmatter is valid | `./.github/scripts/Test-Frontmatter.sh` | a header breaks |
| 2 | Prose is clean | `codespell` | a typo |
| 3 | The template's shell still parses | `./.github/scripts/shell-lint.sh` | the block is malformed |
| 4 | The block works | this plan's own pull request carries a `## Time` section written by it | the algorithm is wrong |

Row 4 is the real proof: this pull request is merged by a script that uses the template as written.

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/guides/pr-script-template.md` | Modify | the section, the merge step, rule 11 |
| `docs/guides/development-process.md` | Modify | the sentence beside the report |

## Related Documents

- [#188](https://github.com/NobleFactor/noblefactor-ops/issues/188) -- this plan; the algorithm and the two commands
- [#157](https://github.com/NobleFactor/noblefactor-ops/issues/157) -- PR tooling: where `star gh pr span` will live
- [pr-script-template.md](../../guides/pr-script-template.md) -- the template this changes
- [development-process.md](../../guides/development-process.md) -- the process this names it in
