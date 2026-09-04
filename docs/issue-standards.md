---
title: "Issue Standards"
type: Process
status: Approved
---

# Issue Standards

Every issue in a NobleFactor repository carries a **kind**, and the kinds form a hierarchy. The
scheme is enforced: `Get-EpicReport --audit` names every issue it cannot place, because an issue the
scheme cannot place is invisible in every other view.

This document is to issues what [documentation-standards.md](documentation-standards.md) is to
documents — the schema a person applies when filing and a checker applies afterwards. The two are
read alongside [guides/development-process.md](guides/development-process.md), which owns how work
is organized once an issue exists.

## The five kinds

| Kind | Tier | What it is |
| --- | --- | --- |
| `epic` | one | A body of work spanning several features |
| `feature` | two | A capability the product gains; the unit an epic is decomposed into |
| `task` | three | Work that **implements part of** a feature |
| `bug` | three | Work that **repairs** a feature |
| `chore` | outside | Work on the **process**, not the product; carries `Epic:Process` |

`task` and `bug` are peers and **mutually exclusive**. An issue carrying both is misclassified, and
this is the most common fault the audit reports. The question that separates them is not difficulty
or size: does this build something the feature did not have, or restore something it was supposed to
have already?

## What a chore is

**A chore is a cleanliness or efficiency task, undertaken to reduce friction by improving or
correcting a human or machine process.**

That is the whole of it, and the emphasis is deliberate: a chore targets **how the work is done**,
never what the software does.

| A chore | Not a chore |
| --- | --- |
| Plan documents are named freely, so a plan cannot be found from its branch | A provider ignores a configured value |
| The report script gains the views the team actually reads | A rename inside a package the product ships |
| A linter is pinned in one place instead of three | Test fixtures that nothing executes |
| A case study lives outside every repository, one `rm` from gone | A deprecated dependency in shipped code |

The right-hand column is product work. Some of it is cleanup and none of it is a chore: tidying code
the product ships is a `task` under the feature that owns that code, or a `bug` if the tidying
repairs something.

**A chore therefore sits outside the `epic → feature → task/bug` hierarchy.** That hierarchy
classifies the product; a chore is not part of the product, so it has no parent feature. It is not a
lesser kind or a leftover — it is a different axis, and it is deliberately not squeezed onto the
product's.

The test, when a chore is hard to distinguish from a task: **if the change shipped to a user, would
they notice?** A chore is invisible to them and visible to everyone who works here.

## The process epic

Chores carry `Epic:Process`, and **that epic never closes.**

Every other epic is a body of work with an end: it decomposes into features, the features ship, and
the epic closes. `Epic:Process` has no such state. There is no release in which friction is finished,
no point at which the tooling stops needing to keep up with the work it serves. It is a standing home
for that work, not a plan to complete it.

That is a deliberate exception to what an epic normally means, and it is stated here so a permanently
open epic reads as designed rather than as one nobody got round to closing.

It also keeps the scheme uniform where it matters. Every issue carries exactly one `Epic:<Name>`
label, and the audit needs no special case for chores — the one exception remains a bug awaiting
triage, which genuinely has no epic yet because it has no feature yet.

A chore still has **no parent feature**, and so carries no `**Feature:**` marker. Under
`Epic:Process` there is no feature tier; the chores hang directly from the epic. That is what "outside
the hierarchy" means in practice: it borrows the epic label so every issue can be placed, and takes
none of the tiers below it.

**Every participating repository provides the `Epic:Process` label**, since chores arise everywhere —
unlike other epic labels, which exist only where their epic is hosted. The epic issue itself lives
once, in the hub: [NobleFactor/noblefactor-ops#142](https://github.com/NobleFactor/noblefactor-ops/issues/142).
Chores in other repositories reference it the way any cross-repository parent is referenced.

## Placement and the two markers

**Every issue carries its epic's label**, `Epic:<Name>` — with the exception below.

**Every tier-three issue names its parent** in its body:

```
**Feature:** #<number>
```

The number alone, or `owner/repo#<number>` when the feature lives in another repository, which
happens: features cross repository boundaries. An issue without the marker is reported as
**Unfiled** rather than being attached to something by guesswork.

### The one exception

**A bug awaiting triage carries no epic.** It has no feature yet, so it has no epic, and the audit
exempts it. That is the triage queue, not a fault.

There is no second exception. A chore carries `Epic:Process`, so the one-epic rule holds for every
kind.

## Triage adds attributes, never a kind

| Attribute | Answers |
| --- | --- |
| `Severity:<level>` | what the impact is if this fires |
| `Priority:<Pn>` | when we act on it |

Neither changes what an issue *is*. An issue has exactly one kind for its whole life; triage adds
and revises attributes around it. `Area:<Name>` is likewise an attribute, naming the part of the
system an issue touches.

## Well classified

An issue is well classified when it carries:

- **exactly one** kind label, and
- **exactly one** `Epic:<Name>` label

Anything else is a fault the audit names: no kind, multiple kinds, no epic, multiple epics. The two
exceptions above apply.

Only **open** issues are audited. Closed ones predate the scheme and cannot be reclassified without
rewriting history nobody will read.

## The labels a repository provides

A participating repository carries all five kinds — `epic`, `feature`, `task`, `bug`, `chore` —
plus `Epic:Process`, an `Epic:<Name>` label for each epic it hosts, and the triage attributes it
uses. `Epic:Process` is required everywhere rather than per-epic, because chores arise in every
repository.

Nothing currently checks this, and the sets have diverged: `noblefactor-ops` carries four kinds and
no `chore`, while `devlore-cli` carries five plus `Area:*`, `Severity:*` and `Priority:*`. A missing
kind is not a decision not to use it; it is a repository where that kind cannot be filed and nobody
noticed.

GitHub's default labels — `enhancement`, `documentation`, `duplicate`, `good first issue`,
`help wanted`, `invalid`, `question`, `wontfix` — are **not kinds**. They look reasonable at filing
time and place nothing. `enhancement` in particular is the trap: it reads as the natural label for
new work and is invisible to the scheme.

## What the audit is for

`Get-EpicReport --audit` opens every run with the classification audit, before any view. That order
is the point: a report that renders a tidy hierarchy while silently omitting the issues it could not
place is worse than no report, because it looks complete.
