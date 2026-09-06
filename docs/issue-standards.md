---
title: "Issue Standards"
type: Process
status: Approved
---

# Issue Standards

Every issue in a NobleFactor repository carries a **kind**, and the kinds form a hierarchy. The
scheme is enforced: `star gh issues audit` names every issue it cannot place, because an issue the
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
| `chore` | three | Work on the **process**, not the product; the tooling axis, `Epic:Ops:Process` |

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

**A chore is on a different axis, not outside the hierarchy.** The `Ops:` segment marks that axis
(below). Within it a chore files exactly as a task does: under a feature, of an epic. What makes it a
chore is what it changes — how the work is done — never where it hangs. An epic of chores needs
features like any other epic, and a chore that names no feature is unfiled like any other tier-three
issue.

The test, when a chore is hard to distinguish from a task: **if the change shipped to a user, would
they notice?** A chore is invisible to them and visible to everyone who works here.

## The process epic

Chores carry `Epic:Ops:Process`, and **that epic never closes.**

Every other epic is a body of work with an end: it decomposes into features, the features ship, and
the epic closes. `Epic:Ops:Process` has no such state. There is no release in which friction is finished,
no point at which the tooling stops needing to keep up with the work it serves. It is a standing home
for that work, not a plan to complete it.

That is a deliberate exception to what an epic normally means, and it is stated here so a permanently
open epic reads as designed rather than as one nobody got round to closing.

It also keeps the scheme uniform where it matters. Every issue carries exactly one `Epic:<Name>`
label, and the audit needs no special case for chores — the one exception remains a bug awaiting
triage, which genuinely has no epic yet because it has no feature yet.

**It needs features like any epic.** Ruled 2026-09-04: an epic with no features is not an epic — it
is ill-defined, or incomplete and awaiting its plan. `Epic:Ops:Process` is no exception. Its
permanence is about *closing*, not about *shape*. A chore files under one of its features and carries
the `**Feature:**` marker like any tier-three issue. At the time of writing
[#142](https://github.com/NobleFactor/noblefactor-ops/issues/142) has three features and eight chores
that name none; the report shows every one as `unfiled, no parent feature`, which is this rule being
enforced.

**Every participating repository provides the `Epic:Ops:Process` label**, since chores arise everywhere —
unlike other epic labels, which exist only where their epic is hosted. The epic issue itself lives
once, in the hub: [NobleFactor/noblefactor-ops#142](https://github.com/NobleFactor/noblefactor-ops/issues/142).
Chores in other repositories reference it the way any cross-repository parent is referenced.

## Placement and the two markers

**Every issue carries its epic's label**, `Epic:<Name>` — with the exception below.

**Every tier-three issue — task, bug, or chore — names its parent** in its body:

```
**Feature:** #<number>
```

The number alone, or `owner/repo#<number>` when the feature lives in another repository, which
happens: features cross repository boundaries. An issue without the marker is reported as
**Unfiled** rather than being attached to something by guesswork.

**The marker is the last `**Feature:**` in the body** — the closing metadata line, conventionally
`**Epic:** #N · **Feature:** #M`. Prose above it may cite the form as an example, as this very
repository's #141 does; the report reads the last one and ignores the rest.

### What carries no epic

**Work carries an epic; a view of the work does not.** Two kinds of issue are not work items:

- **A thread issue** — the narrative of a scenario. Kind `feature`, title beginning `Thread:`,
  carrying its own `Thread:<Name>` label and no `Epic:`. A scenario has no owner; that is the whole
  reason it is a thread and not an epic.
- **A schedule issue** — the declared order of execution. Kind `chore`, title beginning `Schedule:`,
  carrying no `Epic:`. A sequence has no owner either.

Both are placed by their own convention, so the one-epic rule's purpose — that nothing goes
unplaced — is served. Ruled 2026-09-05; before that, thread issues carried the epic "that most
owned the scenario", which was a fiction the rule imposed.

**And one work item carries no epic yet:** a bug awaiting triage. It has no feature, so it has no
epic, and the audit exempts it. That is the triage queue, not a fault.

A chore carries `Epic:Ops:Process`, so the one-epic rule holds for every other work item.

## Three ways to slice the work

The same issues are read three ways, and the three are **peers** — none is above another, and none
is an exception to another:

| | Answers | Its organizing issue | Its label |
| --- | --- | --- | --- |
| **Epic** | who **owns** this | the epic, kind `epic` | `Epic:<Name>`, exactly one per work item |
| **Thread** | what **scenario** this serves | the thread issue, kind `feature`, title `Thread:` | `Thread:<Name>`, zero or more |
| **Schedule** | in what **order** we execute | the schedule issue, kind `chore`, title `Schedule:` | none — lanes are named in the body |

A work item — feature, task, bug, chore — carries exactly one epic and may carry threads. The
organizing issue of a thread or a schedule carries no epic (§What carries no epic). The report
renders each slice: `--by epic`, `--by thread`, `--by schedule`.

## Threads

**A thread is a narrative — a use case or scenario — and it exists to carry cross-cutting feature
development.** Its beats are the steps of a story a person could tell, and its parts land wherever
the machinery lives: usually several epics, often several repositories.

It is a second label family beside `Epic:`, and a second axis rather than a tier:

| | `Epic:<Name>` | `Thread:<Name>` |
| --- | --- | --- |
| Answers | which body of work **owns** this | which scenario this **serves** |
| Cardinality per issue | exactly one | zero or more |
| Crosses repositories | no | yes, by design |
| Closes when | the work ships | the scenario works end to end |

An issue keeps its kind and its epic and gains a thread label. Nothing about the hierarchy changes.

**When to open one.** The test: *would moving this work into one epic take issues away from the epic
that owns their machinery?* If yes, it is a thread. `Thread:WritOrigin` needs a codec fix, a schema
addition, a layer registration and a drift report — four owners, one scenario. Minting
`Epic:WritOrigin` would have stripped each from the epic that owns it.

**The thread issue** is kind `feature`, carrying its own thread label and **no `Epic:` label** — a
scenario has no owner (§The two exceptions). Its body is the narrative, with a beat table naming each
member and the epic that owns it. **The order is the story's; the ownership is the epics'.** Beats are not filed in the order
they execute, so neither issue number nor label gives the order — only the table does.

Two conventions the report reads, so they are rules: **the thread issue's title begins `Thread:`**,
which is how it is told apart from the members that share its label; and **the beat table's first
column is the beat number, and the first issue reference on the row is the member** — `#N` for the
thread issue's own repository, `owner/repo#N` across repositories. Prose may cite issues freely; only
rows whose first cell is a number are read.

**What the audit checks.** No cardinality rule: zero, one or several thread labels are all valid, and
whether a story is fully told is a judgement, not a label property. One agreement rule, once the
tooling implements it: every issue carrying `Thread:<Name>` appears in that thread's beat table, and
every issue in the table carries the label. Both directions are faults. Today nothing checks either —
`star gh issues report --by thread` reads them and reports the agreement faults under each
thread's table; `--audit --unthreaded` lists open work in no thread as a question rather than a fault.

**One namespace.** A thread name must not collide with an epic name. Both families render as sections
of the same report, and `Thread:Process` beside `Epic:Process` is unreadable.

## The Ops axis

**A label whose name begins `Ops:` marks the tooling axis — work on how we work. Absence means
product.**

| Tooling | Product |
| --- | --- |
| `Epic:Ops:Process` | `Epic:ResourceModel` |
| `Thread:Ops:PortableTooling` | `Thread:WritOrigin` |

The segment marks the exception rather than relabelling the majority, so the families are unchanged
and every rule above survives untouched: still exactly one `Epic:`, still zero or more `Thread:`. A
report sections on the prefix; a person scanning a list finds the tooling grouped; and the namespace
rule is satisfied for tooling as a side effect, since an `Ops:` name cannot collide with a product
name.

Tooling labels carry a distinct hue so the visual scan agrees with the machine rule. The hue is a
convenience and proves nothing; the segment is the rule.

## What an empty level means

A level with nothing under it is one of two things, and a report that renders it blank says neither —
blank reads as an error whichever cause it has. The states are named:

| Level | With nothing under it, it is |
| --- | --- |
| An epic with no features | **ill-defined**, or **awaiting plan and design** |
| A feature with no tasks | **awaiting decomposition** |
| A task, bug or chore with no feature | **unfiled** |

Ruled 2026-09-04: **an epic with no features is not an epic.** Either it should not carry the kind, or
its plan has not yet produced the features it will decompose into — and until it has, that is its
state, and the report should say so. `Epic:Ops:Process` is currently in it.

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

- **exactly one** kind label,
- **exactly one** `Epic:<Name>` label — except a bug awaiting triage, and the organizing issue of a
  thread or a schedule, which carry none, and
- **zero or more** `Thread:<Name>` labels — there is no cardinality rule for threads

Anything else is a fault the audit names: no kind, multiple kinds, no epic, multiple epics. Exempt
from the epic rule: a bug awaiting triage, a thread issue, a schedule issue (§What carries no epic).

Only **open** issues are audited. Closed ones predate the scheme and cannot be reclassified without
rewriting history nobody will read.

## The labels a repository provides

A participating repository carries all five kinds — `epic`, `feature`, `task`, `bug`, `chore` —
plus `Epic:Ops:Process`, an `Epic:<Name>` label for each epic it hosts, a `Thread:<Name>` label for
each thread with members there, and the triage attributes it uses. `Epic:Ops:Process` is required
everywhere rather than per-epic, because chores arise in every repository; a thread label is required
wherever the thread reaches, because a thread crosses repositories by design.

`star gh labels audit` checks this across the configured repositories and `star gh labels sync`
creates what is missing and aligns colour and description to the first configured repository that
carries the label. Thread labels are **set-wide**: a thread crosses repositories by design, so a
`Thread:<Name>` present in one configured repository is expected in all of them, or nothing there can
join. `sync` never deletes — a label removal strips it from every issue carrying it. A missing kind is
not a decision not to use it; it is a repository where that kind cannot be filed and nobody noticed,
and the audit now notices.

GitHub's default labels — `enhancement`, `documentation`, `duplicate`, `good first issue`,
`help wanted`, `invalid`, `question`, `wontfix` — are **not kinds**. They look reasonable at filing
time and place nothing. `enhancement` in particular is the trap: it reads as the natural label for
new work and is invisible to the scheme.

## What the audit is for

`star gh issues audit` is the classification audit, and `star gh issues report --view tree` opens
with it before any section. That order
is the point: a report that renders a tidy hierarchy while silently omitting the issues it could not
place is worse than no report, because it looks complete.
