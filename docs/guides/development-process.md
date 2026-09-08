---
title: Development Process
description: How work is organized — one issue at a time, one worktree, a plan before the work, and documents updated on every commit
type: Process
audience: Engineers
status: Approved
created: 2026-09-01
updated: 2026-09-08
---

# Development Process

This document defines how work is organized: what a unit of work is, where it happens, what is written
before it starts, and what must be true of every commit.

It is about **discipline**, not mechanics. Branching strategy, release workflow, rollback and tagging are
in [release-process.md](release-process.md); the two are read together.

---

## Section Status

| Section | Status | Notes |
|---------|--------|-------|
| The unit of work | Approved | Feature spans issues; worktree is a pull request; session is an issue |
| Discovering issues mid-work | Approved | Log, then decide where it is resolved |
| Blocking issues | Approved | Definition and the choice it forces |
| Plans | Approved | Named after the issue; committed before the work begins; every issue ends with a linked table of its plan and design documents |
| Documents on every commit | Approved | No deviations |
| Reference implementation | Approved | `git-open-branch`, `git-close-branch` |

---

## The unit of work

**A branch requires an issue.** A branch names work; work is tracked. A branch attached to nothing is a
claim nobody can find later, and the issue is what makes it findable from the outside.

What that issue must carry — its kind, its place in the hierarchy, and the markers that put it there
— is [issue-standards.md](../issue-standards.md). This document begins where that one ends: it
assumes a well-classified issue and describes how the work around it is organized.

**Three things nest, and they are not the same size.**

| | What it is | How many at once |
|---|---|---|
| **Feature** | A body of work spanning several linked issues | Many, in parallel |
| **Worktree** | One pull request, resolving one or more of that feature's issues | One per pull request |
| **Session** | One issue, worked to completion | One per issue |

**A feature groups the issues that serve it.** Many features may be in flight at once. The feature is
what makes a set of otherwise unrelated issues legible as one intent.

**The report is `star gh issues report`.** By epic, by feature, by thread, or by schedule — the
same issues read four ways — and every pull request ends with it, scoped to what the pull request
served ([pr-script-template.md](pr-script-template.md), rule 9).

**A thread is not a fourth row.** It is a narrative — a use case or scenario — whose beats land in
several features, usually across epics and often across repositories. A session serves a feature; a
thread is what several sessions add up to. It is a view across units of work, not a unit itself, and
it is defined in [issue-standards.md](../issue-standards.md#threads).

**A worktree is a pull request.** It branches from the default branch and resolves one or more of its
feature's issues. As many worktrees may be open as there are pull requests in flight; they do not
compete for attention, because `git-open-branch` names each one `<repo>.<issue>-<name>` and a directory
listing is therefore the whole picture. Legibility comes from naming, not from scarcity.

Within that worktree:

- **More than one issue may be resolved.** The worktree is not limited to the issue that opened it.
- **No pull request until every issue in that worktree is resolved.** A worktree is finished as a whole
  or not at all. Opening a pull request while one of its issues is outstanding either strands that issue
  or drags it into a second branch.

**A session targets a single issue and works it to completion.** Not a worktree's worth of work — one
issue. A session that drifts across issues leaves each of them half-described in the middle of another's
context, and produces a conversation nobody can resume with a clear question in mind.

**A session is disposable and easily resumed.** Closing one is normal and costs nothing: `Start-Claude`
resumes the conversation belonging to that repository and branch. That is what makes a session per issue
affordable. Were resuming expensive, sessions would grow to fit the worktree and stop bounding anything.

**A session declares the feature it serves.** `Start-Claude --issue <feature>` titles it from that
issue, giving `<repository> | <branch> | <feature>`. The branch already names the issue that opened the
worktree, so titling by the feature is what tells parallel sessions apart — with several features in
flight, the session name is the only thing distinguishing one from another at a glance.

A pull request resolves one or more issues. Nothing else is acceptable in it. Work that resolves no issue
is a stray commit, and a stray commit is how a change arrives that nobody can explain six months later.

---

## Discovering issues mid-work

Work uncovers work. That is normal and is not a reason to stop.

**Log the issue when you find it.** Not at the end of the day, and not "if it still matters later" — the
context that made it visible is at its sharpest in the moment it appeared, and a defect described a week
late is a worse description.

**Then decide, immediately, where it will be resolved.** There are exactly two answers, and deferring the
choice is not one of them:

| Decision | What follows |
|----------|--------------|
| **In the current worktree** | Add it to the current plan. It joins the set that must be resolved before this worktree can open a pull request, and it gets its own session — not more work inside the one in hand. |
| **Elsewhere** | Create the plan for that worktree and its issue. Reference that plan and its issue number from the current plan. |

The cross-reference is the point. A plan that mentions a problem without saying where it is handled has
recorded an anxiety, not a decision.

**Then move on**, unless the issue is blocking.

---

## Blocking issues

Severity does not make an issue blocking. Neither does annoyance.

**An issue is blocking when we cannot proceed without addressing it, and it is either:**

- **orthogonal** — a hitherto unknown defect exposed by the current work, belonging to a different
  concern than the one in hand; or
- **large enough to be its own piece of work** — it will take many steps, each of its own commit, and it
  feels like a new piece of work rather than a detour within this one.

A blocking issue forces a choice, and the choice is made explicitly and written down:

1. **Open a new worktree for it**, resolve it, and return. This is right when the issue is orthogonal —
   mixing two unrelated concerns in one branch makes both harder to review and impossible to revert
   independently.
2. **Resolve it in the current worktree**, and delay the pull request until everything in that worktree
   is resolved. This is right when the issue is entangled with the work in hand, where separating them
   would cost more than it clarifies.

Neither answer is the default. What is not permitted is proceeding as though the issue were not there.

---

## Plans

**Every task begins with a plan**, written from [docs/plans/TEMPLATE.md](../plans/TEMPLATE.md) to

```
docs/plans/<type>/<issue>-<name>.md
```

where `<type>` is drawn from the alternation the organization ruleset enforces on every ref — `chore`,
`doc`, `feature`, `fix`, `hotfix`, `refactor`, `release` — and `<issue>-<name>` is what the branch
carries. The plan's path below `docs/plans/` is therefore its branch's name exactly, so neither can be
searched for without finding the other, and the issue number is visible in both.

A plan named for its subject rather than its issue — `reconciliation.md`, `cli-output-conventions.md` —
is reachable only by someone who already knows what it is called.

**The plan is committed before the work begins.** Not alongside it, and not after it. A plan written
afterwards is a description, and a description cannot be reviewed as a proposal — by then the decisions
it would have surfaced have already been made.

The order is: issue, then branch, then plan committed on that branch, then review, then the work.

**Two issues may share a plan.** A worktree may resolve more than one issue, and the second need not
open a plan of its own when the first already describes the work. When it does open one, it is named
for its own issue, so the path rule above bends only in the branch segment: the branch carries the
issue that opened the worktree, and the plan carries the issue it serves.

**A plan carries an `## Issue NNN` heading for every issue it serves.** The number alone — no `#`,
no title. GitHub makes a heading's anchor from its text, so a heading that carries the title breaks
every link to it the moment the title is reworded. The number alone yields `#issue-nnn`, which cannot
change, and a link into a shared plan lands on the section for that issue rather than the top of a
document about something else.

**Every issue ends with a table of its documents.** Headed `## Plan and design documents`, two
columns, `Kind` and `Document`: one row for the plan and one for each design document the issue
bears on. Each row is a link to a committed file, not a title. Design documents are not required — an
issue that touches none carries the plan row alone — but where they exist they are linked, because a
title is something the reader has to go and find.

Links go to the **default branch**, `main` or `develop`. Such a link is dead until the branch merges
and correct forever after. A link to the feature branch is the other way round: correct while the
work is open, then dead when `git-close-branch` deletes the branch — which is the moment the issue
closes and becomes most likely to be read.

An issue whose branch has not opened has no plan yet, and its plan row says so rather than being
omitted. The plan arrives when the branch does, as the order above already places it, and the row is
completed then. Every issue carries the table from the day it is filed; the plan link is the one row
that is expected to start empty.

---

## Documents on every commit

**Every commit updates every document it touches — plan, architecture, and user-facing material. No
deviations.**

This is not a tidiness rule. Stale documentation is worse than absent documentation, because absent
documentation is honest about what it does not tell you. A plan whose phases are marked incomplete after
they have shipped, or an architecture document describing types that were deleted, makes the whole
documentation system untrustworthy: a reader who finds one stale claim must now verify every other claim
against the code, which is the cost the documents existed to avoid.

In particular:

- **Phase status is updated the moment a phase completes**, not batched at the end of the work.
- **A status document that tracks landed-versus-designed is corrected in the commit that changes which is
  which**, not in a later cleanup.
- **A document that describes removed code is corrected or deleted**, not left as history unless it says
  in its own text that history is what it is.

---

## Reference implementation

`git-open-branch` and `git-close-branch` implement the worktree rules above. `git-open-branch` refuses to
create a branch without an issue, derives the branch name from that issue's labels and title so the name
is a consequence of the work rather than of the typist, and creates the worktree. `git-close-branch`
verifies a pull request is merged before it deletes anything, removes the worktree, and deletes the
branch locally and on the remote.

`Start-Claude` implements the session rules. It opens a session named
`<repository> | <branch> | <feature>`, takes the feature from `--issue`, and resumes the conversation
belonging to that repository and branch. It is what makes a session disposable in practice rather than
in principle: without cheap resumption there is no reason to close one, and a session that is never
closed stops bounding a single issue.

The rules in this document are normative on their own. The scripts make them convenient and hard to
forget; they do not define them, and a team without the scripts is held to the same process.

## Where a script lives

The rule, as ruled 2026-09-07 (noblefactor-ops#147):

- Code that takes a dependency on, or contributes to, a repository's stack goes into a directory named
  for that repository: `Home/noblefactor-ops[.<selector>]` for anything that needs the base.
- If not, it goes elsewhere. What is to be deployed unconditionally, wherever you go, goes into
  `common[.<selector>]` — so `common` depends on nothing outside itself.
- Every bash script sources `Declare-BashScript`. It is what gives a script `--help`, a man page and
  exit codes; a script without it is below spec, not portable. So every bash script depends on the base,
  and no bash script lives under `common*`. **A git hook is exempt** (ruled 2026-09-08): git runs it from
  the hooks path, where there is no sibling to source, and a hook is configuration git carries rather
  than a command anyone runs.

writ makes the first bullet enforceable: layers are repositories, a repository's name is a project name,
and a repository-named project is implicit wherever that repository is a configured layer
(devlore-cli#850). `personal/Home/noblefactor-ops.Darwin` therefore deploys only where the base is
registered — a script that sources the base's file cannot land on a machine without it. Consequences:

- **`Declare-BashScript` and the `git-*` commands are the base's `common`.** A git-supporting script
  comes with Windows too — Git for Windows brings bash — so it deploys everywhere git does, and its
  foundation sits beside it in the same `~/.local/bin`. In a consuming layer, a git-supporting script
  that sources `Declare-BashScript` is `noblefactor-ops` with no selector; any other bash script is
  `noblefactor-ops.<selector>`, keeping the selector it had.
- **A context project keeps its scripts, and they source the base's file where they are.** Ruled
  2026-09-08. A project named for nothing configured — a family's, an employer's — is deployed by name
  on top of the whole stack, so the base is present by construction; the dependency is implicit, and
  naming a directory for the base is for dependencies that would otherwise be invisible. The project
  name keeps its own meaning, which is who the scripts are for (personal#177).

Lint follows the dependency it declares. Every consumer carries `# shellcheck source=Declare-BashScript`
— the file's name, never a path — directly above its `source` line, and the gate supplies the directory
with `shellcheck -P`: this repository's `Home/common/.local/bin`, a consumer's deployed `~/.local/bin`,
or in a consumer's CI a checkout of this repository at a pinned ref (`DECLARE_BASHSCRIPT_DIR`).

---

## References

- [release-process.md](release-process.md) — branching strategy, releases, rollback, tagging
- [docs/plans/TEMPLATE.md](../plans/TEMPLATE.md) — the plan every task begins with
- [docs/issue-standards.md](../issue-standards.md) — issue kinds, the hierarchy, and triage attributes
- [docs/documentation-standards.md](../documentation-standards.md) — frontmatter families
- [docs/github-branch-protection.md](../github-branch-protection.md) — protected branches and required checks
- [pr-script-template.md](pr-script-template.md) — the PR script format
