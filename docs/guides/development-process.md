---
title: Development Process
description: How work is organized — one issue at a time, one worktree, a plan before the work, and documents updated on every commit
type: Process
audience: Engineers
status: Approved
created: 2026-09-01
updated: 2026-09-01
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
| The unit of work | Approved | One issue, one worktree |
| Discovering issues mid-work | Approved | Log, then decide where it is resolved |
| Blocking issues | Approved | Definition and the choice it forces |
| Plans | Approved | Named after the issue; committed before the work begins |
| Documents on every commit | Approved | No deviations |
| Reference implementation | Approved | `git-open-branch`, `git-close-branch` |

---

## The unit of work

**A branch requires an issue.** A branch names work; work is tracked. A branch attached to nothing is a
claim nobody can find later, and the issue is what makes it findable from the outside.

**One open worktree at a time.** Not one branch — one worktree. Work happens in one place, and its state
is legible at a glance because there is only one place to look.

Within that worktree:

- **More than one issue may be resolved.** The worktree is not limited to the issue that opened it.
- **No pull request until every issue in that worktree is resolved.** A worktree is finished as a whole
  or not at all. Opening a pull request while one of its issues is outstanding either strands that issue
  or drags it into a second branch.

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
| **In the current worktree** | Add it to the current plan. It joins the set that must be resolved before this worktree can open a pull request. |
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

The rules in this document are normative on their own. The scripts make them convenient and hard to
forget; they do not define them, and a team without the scripts is held to the same process.

---

## References

- [release-process.md](release-process.md) — branching strategy, releases, rollback, tagging
- [docs/plans/TEMPLATE.md](../plans/TEMPLATE.md) — the plan every task begins with
- [docs/documentation-standards.md](../documentation-standards.md) — frontmatter families
- [docs/github-branch-protection.md](../github-branch-protection.md) — protected branches and required checks
- [pr-script-template.md](pr-script-template.md) — the PR script format
