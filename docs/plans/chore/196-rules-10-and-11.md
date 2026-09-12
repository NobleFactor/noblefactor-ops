---
title: "Rules 10 and 11 join the agents' document: a finding is consulted when found; \"works\" means works for the user"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/196
status: complete
created: 2026-09-11
updated: 2026-09-11
---

# Plan: rules 10 and 11

Lanes 2 and 3 of `Schedule: process tooling` (#206), one pull request: #196 and #203.

## Problem statement

`docs/guides/agent-rules.md` holds nine rules, each written the day an agent broke it. Two more were broken
and ruled since, and live only in issue bodies and a session's memory:

- **#196, 2026-09-09.** An agent read the writ guides, found them contradicting themselves, took the finding
  as a licence — rewrote the rule, coded it, changed two guides, edited the plan to match, ticked its boxes.
  Ruled: *"You are not authorized to change design."* Drift between issues, designs, plans and code is
  resolved in real time, with the user, never by the agent choosing a side.
- **#203, 2026-09-08.** *"`--force` works"* was written after testing a scratch copy; the user's command was
  `main`'s and said `unrecognized option`. Then acceptance ran `--dry-run` only, which never reaches the
  live call; the live path failed on first use. Ruled: *"don't tell me something is working when it's
  working in your tree and nowhere else."*

A rule that exists only in an issue is not read by the agent that starts tomorrow. The document is.

## Goals

1. Rules 10 and 11 are in the document, in the shape the first nine use — **The rule. The breakage.
   Compliance.** — with the day each was broken and the words that ruled it.
2. The closing note still holds: a rule is added the day it is broken, and this document holds what the
   instructions assume and an agent still got wrong.
3. Nothing else changes. Two rules, one `updated` date.

## Before → after

| | Before | After |
| --- | --- | --- |
| Rules | 1–9 | 1–11 |
| Rule 10 | in #196's body | *A finding is consulted the moment it is found* — the finding, the delta, the options go to the user before anything else; a finding never justifies a code, guide or plan change in the turn it is found |
| Rule 11 | in #203's body and a session memory | *"Works" means works for the user* — report what is implemented, where it was tested, and what stands between it and the user's PATH; a dry run is not a test of the live path |
| `updated` | 2026-09-08 | 2026-09-11 |

## Implementation Phases

### Phase 1: the plan
- [x] This document, the branch's first commit

### Phase 2: the two rules — complete
- [x] `## 10.` and `## 11.` appended after `## 9.`, before the closing note, each with the three parts
- [x] Frontmatter `updated: 2026-09-11`
- [x] The rule text is the issues' text — the words that ruled it, not a paraphrase. Rule 11's
      **Compliance** is the one part written here rather than in #203: what a plan's acceptance and a PR
      script must do to satisfy the rule

### Phase 3: acceptance — complete
- [x] `Test-Frontmatter.sh` 49 checked, 0 errors; `codespell` clean
- [x] `grep -c '^## ' docs/guides/agent-rules.md` = 11
- [x] The closing note is unchanged and still last

## Files to Create/Modify

| File | Action |
| --- | --- |
| `docs/plans/chore/196-rules-10-and-11.md` | Create |
| `docs/guides/agent-rules.md` | Modify — two rules appended, date bumped |

## Decisions

- **One PR, two issues.** Both are additions to one document in one shape; splitting them would be two PRs
  editing the same file for no reason. The schedule allows a PR to close several lanes.
- **The rulings are quoted, not summarised.** The document's own preface: "each carries the day it was
  broken and the words that ruled it."

## Related Documents

- #196, #203; #206 (the schedule); #185 and #193 (how rules 1–9 arrived); `docs/guides/agent-rules.md`

## Open Questions

None.
