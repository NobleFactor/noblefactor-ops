---
title: "The process documents never define a lane or a schedule; the scheme is ruled and unwritten"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/210
status: in-progress
created: 2026-09-12
updated: 2026-09-12
---

# Plan: the documents state the scheme

Lane 5 of `Schedule: process tooling` (#206).

## Problem statement

The scheme was ruled on 2026-09-10 and 2026-09-12. The documents do not carry it. Measured, not assumed:

| Document | The word "lane" | A Schedules section |
| --- | --- | --- |
| `issue-standards.md` | **once**, in a table cell — "none — lanes are named in the body" | **none.** Threads has 44 lines |
| `development-process.md` | **zero times** | none; the unit of work is feature → worktree → session |

So an agent reads the standards, finds threads described in full and schedules named once in passing, and has
nothing that says what a lane is, what a schedule commits to, or how the two differ from the epic tree.

**#171 states it wrongly, and nothing catches that.** *Schedule: the four lanes* says in its own body *"a
lane **is** an epic or a feature"*, and its four lanes are epics and features. Under the scheme a lane is a
**leaf node in the epic hierarchy — a bug, a chore, or a task**. There is one definition, not two shapes.
The report accommodates the error rather than reporting it: `resolve_lane` aggregates an epic, rolls up a
feature, and only then falls through to *"anything else reports its own state"* — so a schedule built the
wrong way renders plausibly and nothing complains.

The documents are the reason that survived: they never defined the word, so there was nothing for #171 to
contradict.

This issue's own first version got that wrong: it attributed #171's sentence to `issue-standards.md`, which
never contained it. The document is silent, not mistaken. That is a writing job, not a correction.

## Goals

1. **A lane is defined** — a task, a chore, or a bug — and connected to the tier-three kinds the document
   already lists.
2. **Schedules get the section threads have**: what a schedule is, what its issue looks like, what the body's
   table must contain for the report to read it, and what the audit does and does not check.
3. **The commitment is written down**: lanes in declared order, numbered 1..N with no gaps, a closed lane
   keeping its number; a pull request may close several; no lane split or added once committed — the owner
   may amend, the agent may not.
4. **The deliverable is written down**: before → after, and that a change to it is a replan.
5. **#171 is named as wrong**, not as an alternative — its lanes are epics and features, which are not lanes.
6. **Rule 9's retirement is honoured in this document** — `development-process.md` restates it and must stop.

## Before → after

| | Before | After |
| --- | --- | --- |
| "lane", `issue-standards.md` | one table cell, undefined | defined beside the five kinds: tier three *is* a lane |
| Schedules, `issue-standards.md` | no section | a section beside Threads: the issue, the table the report reads, the commitment, the deliverable |
| #171 | says "a lane is an epic or a feature"; nothing contradicts it | the documents define a lane as a leaf; #171 is a defect to be fixed, not a shape to copy |
| `development-process.md` | feature → worktree → session; no schedule | the schedule named as what an agent executes across features |
| Rule 9 | "every pull request ends with it" in **both** documents | retired here; #144 retires it in the template |
| The report's axes | "by epic, by feature, by thread, or by schedule" | unchanged — already correct |

## Implementation Phases

### Phase 1: the plan
- [x] This document, the branch's first commit

### Phase 2: `issue-standards.md`
- [ ] The five-kinds table gains the word: tier three — `task`, `bug`, `chore` — is a **lane**
- [ ] A `## Schedules` section beside `## Threads`, in the same shape: what it is, its issue, the table the
      report reads, the commitment, the deliverable, what the audit checks
- [ ] #206 named as the exemplar; #171 named as carrying a wrong definition, with a note that its lanes are epics and features

### Phase 3: `development-process.md`
- [ ] The schedule named as what an agent executes — a sequence of lanes drawn from many features
- [ ] The "every pull request ends with it" sentence corrected: a report is asked for

### Phase 4: acceptance
- [ ] `grep -c lane docs/issue-standards.md` is no longer 1, and the word is defined before it is used
- [ ] A reader can answer, from the documents alone: what is a lane; what may a PR close; may a lane be
      split; what does a schedule promise; why #171 is wrong
- [ ] `Test-Frontmatter.sh`, `codespell`, the repository gate: clean

## Files to Create/Modify

| File | Action |
| --- | --- |
| `docs/plans/chore/210-documents-state-the-scheme.md` | Create |
| `docs/issue-standards.md` | Modify — the lane definition and the Schedules section |
| `docs/guides/development-process.md` | Modify — the schedule, and rule 9's retirement |

## Decisions

- **#171 carries a wrong definition.** Its body says a lane is an epic or a feature, and its four lanes are.
  That is not a second legitimate shape — a lane is a leaf. Correcting #171 itself is not this lane's work
  (it is a schedule issue, and its owner may want it restated or retired), but the documents will name it so
  no one copies it. Raised as a finding rather than acted on.
- **The template is not touched here.** Rule 9 lives in `pr-script-template.md` too, and that whole guide is
  rewritten by #144 (lane 7). Editing it here would collide with that rewrite.
- **No tooling change.** The report already accepts both shapes; the columns it prints for a leaf schedule
  are wrong, and that is a separate, unfiled item the owner has not yet placed. This lane is documents only.

## Related Documents

- #210; #206 (this schedule, the agent shape); #171 (the program shape); #144 (lane 7, the template);
  #156 (the feature); `docs/issue-standards.md`; `docs/guides/development-process.md`

## Open Questions

None.
