---
title: "Issue standards: write down the hierarchy a script already enforces"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/141
status: complete
created: 2026-09-04
updated: 2026-09-04
---

# Plan: issue standards

## Issue 141

## Summary

The organization classifies every issue into five kinds across three tiers, enforces it with
`Get-EpicReport --audit`, and documents it in one script's header comment. This plan writes it down
as `docs/issue-standards.md`, a normative document at the docs root beside
`documentation-standards.md`.

## Current State

| | Where it lives |
| --- | --- |
| The scheme | the header comment of `devlore-cli/scripts/Get-EpicReport` |
| The enforcement | `Get-EpicReport --audit`, in the same repository |
| The documentation | nowhere |

`development-process.md` covers the unit of work, plans, and documents-on-every-commit, and says
nothing about issue kinds, the parent marker, or triage attributes. Nothing in `docs/` mentions
them.

Three consequences, all observed rather than predicted:

- **Filing it wrong is silent.** Four `devlore-cli` issues were filed this week with GitHub's default
  `enhancement` and `documentation` labels. Both are reasonable-looking and neither is a kind. The
  audit flags them; nothing flagged them at filing time.
- **The label sets have diverged.** This repository carries `epic`, `feature`, `task`, `bug` and no
  `chore`, plus eight unused GitHub defaults. `devlore-cli` carries all five plus `Area:*`,
  `Epic:*`, `Severity:*`, `Priority:*`.
- **It already spans repositories.** A `devlore-cli` issue names
  `**Feature:** NobleFactor/noblefactor-ops#140`, so parent links cross repository boundaries.

## Goals

1. The scheme is discoverable without reading a script.
2. A repository's required label set is stated, so divergence is a visible omission.
3. `development-process.md` points at it, since the two are read together.

## Requirements

### Requirement 1: A normative document, not a guide

`docs/issue-standards.md`, at the docs root. `guides/` holds process and style; this is a schema a
human reads before filing and a checker applies afterwards — the same role
`documentation-standards.md` plays for documents, and the same enforcement shape:

| Document | Defines | Enforced by |
| --- | --- | --- |
| `documentation-standards.md` | frontmatter families a document must carry | `Test-Frontmatter.sh` |
| `issue-standards.md` | kinds, tiers and markers an issue must carry | `Get-EpicReport --audit` |

### Requirement 2: State the scheme completely

- The five kinds, each with what it is *for* rather than only its name: `epic`, `feature`, `task`,
  `bug`, `chore`
- The three tiers and which kinds occupy them
- `task` and `bug` as peers, mutually exclusive — the audit's "multiple kind labels" error is
  currently the second most common failure
- `Epic:<Name>` on every issue
- `**Feature:** #<number>` on every tier-three body, and that its absence means "Unfiled" rather than
  a guess
- Triage attributes — `Severity:<level>`, `Priority:<Pn>` — as attributes, never kinds
- "Well classified" defined exactly: one kind label, one `Epic:<Name>` label
- That only open issues are audited, and why: closed ones predate the scheme

### Requirement 3: State the label set a repository must provide

The divergence is invisible today because nothing says what is required. The document lists the
labels a participating repository provides, which turns a missing `chore` from an accident into a
gap someone can see.

### Requirement 4: Describe, do not redesign

The scheme is in use and enforced. This plan documents what exists. Anything that looks wrong while
writing it — the unused GitHub defaults, the missing `chore` here — is recorded as an observation
for a separate issue, not corrected in passing.

## Implementation Phases

### Phase 1: Write the document (status: complete)

`docs/issue-standards.md`, frontmatter per `documentation-standards.md`. Source of truth is the
`Get-EpicReport` header plus the behaviour of `--audit`, read together so the document describes what
the tool does rather than what its comment says it does.

### Phase 2: Reference it from the process guide (status: complete)

`development-process.md` gains a pointer where it introduces the unit of work, and
`## References` gains a row.

### Phase 3: Verify (status: complete)

- The document's frontmatter passes `Test-Frontmatter.sh`
- Every rule in it is checked against `--audit`'s actual behaviour, not only the header comment
- The kinds listed match the labels `Get-EpicReport` accepts
- Spelling passes `codespell` with the repository's ignore file

## Verification

Every rule was checked against `--audit`'s behaviour rather than the header comment, because a
comment being the only specification is the problem this document exists to fix.

| Documented | Implementation |
| --- | --- |
| Five kinds | the jq filter accepts exactly `epic`, `feature`, `task`, `bug`, `chore` |
| `**Feature:**` drives placement, not classification | `parentFeature` feeds `$placement`; it is absent from the faults list |
| Only open issues are audited | `faults: (if .state == "OPEN" then faults else [] end)` |
| A bug in triage is the one epic exemption | `awaitingTriage: isBug and (epicTags \| length) == 0` |
| Frontmatter | `Test-Frontmatter.sh` clean |
| Spelling | `codespell` clean |

## Resolved: where `chore` sits

Outside the `epic → feature → task/bug` hierarchy, which classifies the *product*, because a chore
targets the *process*: a cleanliness or efficiency task undertaken to reduce friction by improving or
correcting a human or machine process.

A chore therefore has no parent feature and carries no `**Feature:**` marker. It does carry
`Epic:Process` — [noblefactor-ops#142](https://github.com/NobleFactor/noblefactor-ops/issues/142) —
a standing epic that **never closes**, because there is no release in which friction is finished.
That keeps the one-epic invariant whole: the audit needs no second exception, and a permanently open
epic is recorded as designed rather than read as an oversight.

The plan's open question is answered by decision, not by discovery: the audit would have flagged an
epic-less chore, and the two candidate fixes were to exempt chores or to give them an epic. The
second was chosen.

Applied to this issue first: #141 was filed as `task` and reclassified `chore` + `Epic:Process`,
which is the standard's first use, on itself.

## Out of Scope

- **Reclassifying the misfiled issues.** devlore-cli#800–#804 need kind labels and a feature tier;
  that is work in that repository and follows this document rather than preceding it.
- **Provisioning labels.** Adding `chore` here, or removing the unused defaults, is a change to
  repository configuration and belongs to its own issue.
- **Changing the scheme.** Requirement 4.

## Open Questions

None. The `chore` question is resolved above.
