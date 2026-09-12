---
title: "git-close-branch: remove_branch removes every trace of a branch, so one fact stops being restated at three call sites"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/204
status: in-progress
created: 2026-09-11
updated: 2026-09-11
---

# Plan: remove_branch

Lane 4 of `Schedule: process tooling` (#206). Re-filed from personal#185, which holds the design discussion.

## Problem statement

`git close-branch` has one model, in one sentence: **remove every trace of a branch** — remote, local, and
the worktree if one holds it. The code does not say that. The three traces are three independent commands
in a 160-line loop, each with its own guards, and a second 80-line loop re-derives the same decisions to
print a preview. So any fact about *the branch* — `--force`, a dirty worktree, deletability — is restated at
each site: `force` is consulted at lines 357, 374, 460, 493 and 501; whoever adds a fact threads it through
two of the three.

That is not a hypothesis; it is the changelog. Six fixes, one seam: personal#124, #148, #153, #154,
#181/#182, #183 — and #186 after this issue was first filed. Three of them are the worktree/branch pair
specifically. The seventh is visible today: line 520's comment says a failed remote delete "leaves BOTH
halves intact" — but the worktree was removed thirty lines earlier. #154 established a real invariant and
reasoned it over two traces when there are three.

And the man page's step 4 promises a fourth order — *"removes its worktree, deletes it locally, then deletes
it on the remote"* — which matches neither the code nor the ruling. Three documents, three orders.

## Goals

1. **One function owns a branch end to end**: survey, decide, remove, report. `--force` is read in one place.
2. **Nothing is removed until everything is decided.** A refusal touches no trace. Today a no-force close of
   a cherry-picked branch removes the worktree and then fails the branch delete.
3. **The order is the safe one, and it is stated once**: remote → worktree → local. The remote is the trace
   that fails for reasons unrelated to the branch and must leave the others intact; the worktree is the one
   irreversible act and follows the flaky step; the local ref goes last because `git branch -d` refuses a
   branch a worktree holds.
4. **The dry run prints the record the live run acts on.** It stops being a second implementation. The class
   of defect where preview and run disagree (#183, #128) becomes unwritable.
5. **The man page says what the code does.**

## Before → after

**Structure**

| | Before | After |
| --- | --- | --- |
| Decision sites | `force` at 5 lines, `dirt` at 3, `delete_flag` at 2 (one per loop) | one `survey_branch`; `--force` read once |
| Removal | three commands inline in the live loop, each guarded | one `remove_branch` that does what the record says |
| Preview | its own 80-line loop re-deriving the decision | prints the record `survey_branch` returns |
| Order | worktree → remote → local (code); worktree → local → remote (man page) | remote → worktree → local (code and man page) |
| A refusal | may land after the worktree is gone | lands before anything is touched |

**Behaviour, on the cases the fixes came from** (throwaway repository; before = deployed `develop`)

| Case | Before | After |
| --- | --- | --- |
| merged branch, clean linked worktree | closed | closed — unchanged |
| cherry-picked branch, no `--force` | worktree removed, then `git branch -d` refused; branch survives with no worktree | refused up front; **nothing touched**; message names `--force` |
| cherry-picked branch, `--force` | closed (#186) | closed — unchanged |
| dirty worktree, no `--force` | refused before removal (#181) | refused; unchanged |
| dirty worktree, `--force` | removed, count reported (#181) | removed, count reported — unchanged |
| remote-only branch | remote deleted | remote deleted — unchanged |
| remote delete fails | worktree already gone; branch alive both sides | **all three traces intact**; one message; re-run when the cause is fixed |
| `--dry-run` of each | a separate derivation | the same record, printed |

## The record

`survey_branch <branch> <record>` fills an associative array by nameref (bash ≥ 4.3; `declare -A` already
requires ≥ 4.0; Darwin ships 5.3, Git for Windows 5.2):

| Field | Meaning |
| --- | --- |
| `local_branch` | `refs/heads/<branch>` exists — set independently of the worktree, because `git worktree remove` does not delete the ref |
| `remote_branch` | origin has the branch |
| `worktree_path` | a linked worktree holds it |
| `dirt_count` | changed + untracked entries in that worktree, counted *before* removal — afterwards there is nothing to count |
| `branch_flag` | `delete_flag`'s answer; `-D` under `--force` |
| `refusal` | why the close will not proceed, or empty. Set when: no local and no remote; `delete_flag` refused and no `--force`; worktree dirty and no `--force`; main-worktree switch was blocked |

`--force` resolves refusals **inside the survey**. A refusal leaves `refusal` set and `remove_branch` does
nothing — "blindly do what is set" is only safe when nothing set is undecided.

`delete_flag`, `forced_cost`, `worktree_dirt`, `worktree_for` are unchanged; the survey calls each once.

## Implementation Phases

### Phase 1: the plan
- [x] This document, the branch's first commit

### Phase 2: `survey_branch` and `remove_branch`
- [ ] `survey_branch` produces the record above; the only reader of `force`, `delete_flag`, `worktree_dirt`
- [ ] `remove_branch` acts on the record: remote, worktree, local — each skipped when unset; stops at the
      first failure and reports which trace failed and what was already done
- [ ] `report_branch` renders a record as the preview line (dry run) or as the closing note (live)

### Phase 3: the two loops become one
- [ ] The dry-run loop becomes: survey each, print each, exit
- [ ] The live loop becomes: switch the main worktree if needed (unchanged, it is about the main worktree,
      not a branch's traces), then survey each, then remove each, then report. The `blocked` map feeds
      `refusal`
- [ ] Line 520's "BOTH halves" comment and its neighbours go with the code they described

### Phase 4: the man page
- [ ] Step 4 states remote → worktree → local and that nothing is removed until every check has passed
- [ ] `--force` and `--dry-run` describe the record, not the old paths

### Phase 5: acceptance, live
- [ ] The behaviour table above, every row, in a throwaway repository — including the remote-failure row
      (origin re-pointed at a path that does not exist) with all three traces verified intact afterwards
- [ ] `shell-lint.sh` clean; `mandoc -T lint` clean
- [ ] **The PR script's own `git close-branch`** — the merged command closing the branch that changed it,
      on the user's PATH, with the deployed copy verified to carry `survey_branch`

## Files to Create/Modify

| File | Action |
| --- | --- |
| `docs/plans/chore/204-remove-branch.md` | Create |
| `Home/common/.local/bin/git-close-branch` | Modify — the refactor |
| `Home/common/.local/share/man/man1/git-close-branch.1` | Modify — the order and the flags |

## Decisions

- **The main-worktree switch stays outside `remove_branch`.** It is a property of the main worktree, done
  once for every selected branch that lives there; a branch's record only learns whether it was blocked.
- **Behaviour is preserved everywhere the fixes established it** — the six rows marked *unchanged*. The two
  rows that change are the two the seam still gets wrong. A refactor that changed a third row would be a
  design change, and that is the user's to make (Rule 10).
- **Remote → worktree → local, not worktree-last.** `git branch -d` refuses a branch a worktree holds, so
  "worktree last" is not available; the worktree goes second, after the step most likely to fail for
  unrelated reasons.
- **Nameref, not globals or a delimited line.** It is the one bash feature that makes a record a record; the
  floor it needs is already met on every platform the script runs on.

## Related Documents

- #204; personal#185 (the design record); #206 (the schedule); personal#124, #148, #153, #154, #181, #183,
  #186 (the fixes the seam produced); #146 / personal#180 (why the file is here)

## Open Questions

None.
