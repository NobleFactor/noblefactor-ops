---
title: "Three rules ruled on 2026-09-08 join the agents' document"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/193
status: draft
created: 2026-09-08
updated: 2026-09-08
---

# Plan: Three rules ruled on 2026-09-08 join the agents' document

## Summary

`docs/guides/agent-rules.md` holds the rules whose only audience is a coding agent, each carrying the day it
was broken. Three more were ruled on 2026-09-08 and live only in one session's memory, where no future
session can read them: nothing may be written to a repository while a pull request is in flight; a branch
found blocked is deleted rather than parked; and a violation is disclosed as a violation rather than
mentioned as state. They become rules 7, 8 and 9, in the shape the existing six use. This is the plan
document required before the work, and it is this branch's first commit — itself the third ruling of the
day, applied.

## Goals

1. **The three rules are readable by a session that was not there**, each with its breakage, the words that
   ruled it, and how to comply.
2. **Nothing else changes.** The engineers' documents stay the engineers'; these are not rules an engineer
   needs.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| `docs/guides/agent-rules.md` | six rules, audience `Claude Code, Codex` | shipped in #186; each rule states the breakage that produced it |
| The three new rulings | in one session's memory only | `nothing-while-a-pr-is-in-flight`, `blocked-branch-gets-nuked`, `disclose-process-violations-immediately` |
| `development-process.md` | correct as it stands | must not absorb these |

## Requirements

Three rules, in the file's established shape — **The rule**, **The breakage**, **Compliance** — appended
after rule 6 and before the closing paragraph.

**Rule 7 — Nothing while a pull request is in flight.** From the moment a PR script starts until its pull
request merges and its branch closes: answer questions, read, measure, explain, and write nothing to any
repository. No plan, no branch, no worktree, no edit inside a checkout. Broken 2026-09-08 by opening a
second worktree while #863's pull request sat in its CI gate. Ruled: questions may be answered while a pull
request is in flight; no code changes, full stop.

**Rule 8 — A blocked branch is nuked, not parked.** The moment a branch's work is found blocked, remove the
worktree and delete the branch; reopen fresh when the blocker clears. Diff any uncommitted content against
its source first, so the discard is knowingly empty. Broken 2026-09-04 through 2026-09-08:
`chore/146-relocate-process-commands`, opened and found blocked within the hour, left parked four days and
21 commits behind. Ruled: "nuke the branch."

**Rule 9 — A violation is disclosed as a violation.** In the next message, named as a breach, with the rule
it broke and the date it started, before anything else and never folded into a status list. Broken across
those same four days: the parked worktree was reported as "the worktree holds twelve files", including in a
message that had been told to hide nothing. Ruled: that is a lie by omission, and it will not be tolerated.

## Implementation Phases

### Phase 1: the plan

- [x] This document, committed as the branch's first commit, per the ruling of 2026-09-08

### Phase 2: the three rules

- [ ] Rules 7, 8 and 9 in `docs/guides/agent-rules.md`, each with its breakage and what was ruled, plainly
- [ ] Rule 5's breakage loses its profanity, on the same ruling
- [ ] The file's `updated` frontmatter moves to 2026-09-08
- [ ] **Acceptance:** the frontmatter gate and codespell pass; `development-process.md` is untouched;
      a reader who was not there can tell what was done wrong and what to do instead

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/plans/chore/193-agent-rules-in-flight-blocked.md` | Create | this plan, the branch's first commit |
| `docs/guides/agent-rules.md` | Modify | rules 7, 8, 9; and rule 5's quote, cleaned |

## Decisions

- **The plan is the first commit.** Ruled 2026-09-08: "You can commit the plan to the branch you will work
  in. That plan should be that branch's first commit." So the history shows what was planned against what
  was built, and a plan cannot be written to fit the work after the fact.
- **Three rules, not one about discipline.** They failed separately and would be complied with separately:
  one governs a window of time, one an event, one a duty to speak.
- **The rulings are paraphrased where what was said was heated.** Ruled 2026-09-08: profanity belongs to the
  conversation after a rule is broken, not to the document that records it. The reader of this guide broke
  nothing and should not meet heat aimed at someone else; accountability happens in the conversation, and the
  document keeps the rule and the breakage. Rule 5 is corrected on the same principle, since it still carries
  a phrase from 2026-09-07.
- **No new rule about worktree hygiene in general.** Rule 7 and rule 8 cover the two ways it actually broke;
  a broader rule would be one nobody violated.

## Related Documents

- Issue #193; feature #156, epic #142
- #185 and #186 — the agents' document and the ruling that created it
- #147 — the work during which all three were broken; personal#177 and devlore-cli#863 — the pull requests in flight at the time

## Open Questions

None.
