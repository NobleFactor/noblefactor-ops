---
title: "Threads, the Ops axis, and what an empty level means"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/148
status: in-progress
created: 2026-09-04
updated: 2026-09-05
---

# Plan: Threads, the Ops axis, and what an empty level means

## Summary

Four rulings were made on 2026-09-04 and none of them is written down. Labels are already live on
both repositories under all four, so the scheme and its documentation have diverged — and
`docs/issue-standards.md` now states, in one clause, the opposite of what was ruled. This plan
records all four in that document, places the thread in `development-process.md`, and names the work
each ruling creates elsewhere.

## The four rulings

1. **A thread is a narrative** — a use case or scenario — and it exists to carry cross-cutting
   feature development. Its parts land in different epics, and often in different repositories.
2. **An epic with no features is not an epic.** It is an ill-defined issue, or an incomplete one
   awaiting its plan and design. The same applies down the tree: a feature with no tasks is awaiting
   decomposition, and a task, bug or chore with no feature is unfiled.
3. **The `Ops:` segment marks the tooling axis.** `Epic:Ops:Process`, `Thread:Ops:PortableTooling` —
   work on *how we work*. Absence of the segment means product work.
4. **Thread and epic names share one namespace.** Different label families, one pool of names,
   because the report renders both as sections and `Thread:Process` beside `Epic:Process` is
   unreadable.

## Goals

1. **Write the four rulings** into `docs/issue-standards.md`, plainly enough that a person filing an
   issue and a script auditing one reach the same answer.
2. **Repair the contradiction** ruling 2 creates with the chore section as written.
3. **Place the thread** relative to the feature in `development-process.md` — it is not a new unit of
   work; it is a view across several.
4. **Name the follow-on work** each ruling creates, without doing it here.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| `Thread:WritOrigin` | Live | 8 members, 5 epics, 2 repositories — devlore-cli#806 |
| `Thread:Ops:PortableTooling` | Live | 10 members, 3 epics, 2 repositories — #152 |
| `Epic:Ops:Process` | Live | 12 issues; replaced `Epic:Process`, which is deleted |
| Thread definition in `issue-standards.md` | **Missing** | Document names five kinds and one label family |
| `Ops:` axis in `issue-standards.md` | **Missing** | |
| Empty-level states in `issue-standards.md` | **Missing** | |
| The chore section | **Contradicts ruling 2** | See below |
| Placement in `development-process.md` | **Missing** | Unit of work is the feature; threads unmentioned |
| `Get-EpicReport --by thread` | **Inert** | Selects epics by name from a hardcoded list of four |

### The contradiction to repair

`docs/issue-standards.md` currently says:

> A chore still has **no parent feature**, and so carries no `**Feature:**` marker. Under
> `Epic:Process` there is no feature tier; the chores hang directly from the epic.

Ruling 2 says the opposite: an epic with no features is not an epic, so `Epic:Ops:Process` needs
features and its chores file under them. That clause goes.

**The evidence is already visible.** With devlore-cli#809 merged, the report renders #142 as twelve
rows in which every one of its eight chores reads `unfiled, no parent feature`. That is the ruling
being enforced, and the document still says it should not be.

## Requirements

### Threads

**A thread is a narrative — a use case or scenario — and it exists to carry cross-cutting feature
development.** Its beats are steps in a story a person could tell; its parts land wherever the
machinery lives.

| | `Epic:<Name>` | `Thread:<Name>` |
| --- | --- | --- |
| Answers | which body of work **owns** this | which scenario this **serves** |
| Cardinality per issue | exactly one | zero or more |
| Crosses repositories | no | yes, by design |
| Closes when | the work ships | the scenario works end to end |
| Audited | exactly one, one exception | label and beat table must agree |

**The filing test:** *would moving this work into one epic take issues away from the epic that owns
their machinery?* If yes it is a thread. devlore-cli#473 belongs with layer pinning whether or not any
scenario needs it; an `Epic:WritOrigin` would have stolen it.

**The thread issue** is kind `feature`, filed under the epic that most owns the scenario, carrying its
own thread label. Its body is the narrative, with a beat table naming each member and the epic that
owns it. **The order is the story's; the ownership is the epics'.**

### The Ops axis

A label whose name begins `Ops:` marks the tooling axis — work on how we work. Absence means product.

- `Epic:Ops:Process`, `Thread:Ops:PortableTooling` — tooling.
- `Epic:ResourceModel`, `Thread:WritOrigin` — product.

It marks the exception rather than relabelling the majority, so the label families are unchanged and
"exactly one epic, zero or more threads" survives untouched. Tooling labels carry a distinct hue so
the visual scan matches the machine rule.

### What an empty level means

The report must say **why** a level is empty rather than rendering a blank section, because empty
reads as an error whichever cause it has.

| State | Means |
| --- | --- |
| Epic with no features | ill-defined, or **awaiting plan and design** |
| Feature with no tasks | **awaiting decomposition** |
| Task, bug or chore with no feature | **unfiled** |

### One namespace for two families

A thread name must not collide with an epic name, and vice versa. The `Ops:` segment resolves this
for tooling work as a side effect; for product work it is a convention the audit can check.

### What the audit checks about threads

**No cardinality rule.** Zero, one or several thread labels are all valid, and a thread's
*completeness* is a judgement about a narrative, not a property of a label set.

**One agreement rule**, once tooling implements it: every issue carrying `Thread:<Name>` appears in
that thread's beat table, and every issue in the beat table carries the label. Both directions are
faults.

Today nothing checks either, because `Get-EpicReport` does not read `Thread:` labels at all. The
document says so rather than implying an enforcement that does not exist.

## Implementation Phases

### Phase 1: `issue-standards.md` — complete

- [x] Add `## Threads` — definition, the two axes, the filing test, the thread issue's shape
- [x] Add `## The Ops axis` — the segment, what its absence means, the hue convention
- [x] Add `## What an empty level means` — the three states
- [x] **Amend `## What a chore is`** — remove the "no feature tier / hang directly from the epic"
      clause, which ruling 2 contradicts
- [x] Amend `## The process epic` — it is `Epic:Ops:Process` now, and it needs features like any epic
- [x] Extend `## Well classified` — `Thread:` has no cardinality rule; state why
- [x] Extend `## The labels a repository provides` — the `Thread:<Name>` family and the `Ops:` segment

**Files**: `docs/issue-standards.md` — Modify

### Phase 2: `development-process.md` — complete

- [x] Place the thread beside the feature in `## The unit of work`: a thread is not a unit of work,
      it is a view across several
- [x] Cross-reference `issue-standards.md` rather than restating the definition

**Files**: `docs/guides/development-process.md` — Modify

### Phase 3: Name the follow-on work

- [ ] **#142 must be decomposed.** Ruling 2 makes it an incomplete epic: twelve issues, three
      features, eight chores all unfiled. Its chores cluster into issue standards (#141, #148), PR
      tooling (#144, #145), process commands and their dependency (#146, #147, #150), and the
      extensions replacing scripts (#140, #151). File as its own chore.
- [ ] **#140's body predates its comments.** It still declares `report: threads: [...]` and
      `--repo <owner/name>`, both superseded by three comments beneath it. Rewrite the body so an
      implementer reads the current design first.
- [ ] **The PR script template has a latent bug.** `cd <worktree>` … `git close-branch` …
      `git status --short` fails with exit 128 every time, because close-branch removes the worktree
      the script is standing in. Observed on PR #811. File as its own chore.

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/plans/chore/148-document-threads.md` | Create | This plan |
| `docs/issue-standards.md` | Modify | All four rulings, and the contradiction repaired |
| `docs/guides/development-process.md` | Modify | Where a thread sits relative to the feature |

## Related Documents

- Issue #148 — this chore
- Issue #142 — `Epic:Ops:Process`, which never closes and currently has no features
- Issue #152 — `Thread:Ops:PortableTooling`, the worked example filed here
- Issue #140 — the star extension that must implement all of this
- `NobleFactor/devlore-cli#806` — `Thread:WritOrigin`, the first thread
- `NobleFactor/devlore-cli#809` — the report renders chores; merged, and its output is the evidence
  for ruling 2
- `docs/plans/doc/141-issue-standards.md` — the plan that wrote the document this one amends

## Open Questions

- [ ] **Does a thread member carry a `**Thread:** #N` body marker, parallel to `**Feature:**`?**
      **Proposed: no.** The `**Feature:**` marker exists because a feature's children are not
      otherwise discoverable; a thread's are, from its own beat table. A marker would be a third copy
      of the same fact to keep in sync.
- [ ] **Is the kind `bug` or `fix`?** The document says `bug`. Usage on 2026-09-04 said "task, fix,
      or chore" more than once. Settle before the extension hard-codes either.
