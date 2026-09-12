---
title: "git-close-branch: remove_branch removes every trace of a branch, so one fact stops being restated at three call sites"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/204
status: complete
created: 2026-09-11
updated: 2026-09-11
---

# Plan: remove_branch

Lane 4 of `Schedule: process tooling` (#206). Re-filed from personal#185, which holds the design discussion.

## Problem statement

`git close-branch` has one model, in one sentence: **remove every trace of a branch** — remote, local, and
the worktree if one holds it. The code does not say that. The three traces are three independent commands
in a 160-line loop, each with its own guards, and a second 80-line loop re-derives the same decisions to
print a preview. So any fact about _the branch_ — `--force`, a dirty worktree, deletability — is restated at
each site: `force` is consulted at lines 357, 374, 460, 493 and 501; whoever adds a fact threads it through
two of the three.

That is not a hypothesis; it is the changelog. Six fixes, one seam: personal#124, #148, #153, #154,
#181/#182, #183 — and #186 after this issue was first filed. Three of them are the worktree/branch pair
specifically. The seventh is visible today: line 520's comment says a failed remote delete "leaves BOTH
halves intact" — but the worktree was removed thirty lines earlier. #154 established a real invariant and
reasoned it over two traces when there are three.

And the man page's step 4 promises a fourth order — _"removes its worktree, deletes it locally, then deletes
it on the remote"_ — which matches neither the code nor the ruling. Three documents, three orders.

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

|                | Before                                                                 | After                                              |
| -------------- | ---------------------------------------------------------------------- | -------------------------------------------------- |
| Decision sites | `force` at 5 lines, `dirt` at 3, `delete_flag` at 2 (one per loop)     | one `survey_branch`; `--force` read once           |
| Removal        | three commands inline in the live loop, each guarded                   | one `remove_branch` that does what the record says |
| Preview        | its own 80-line loop re-deriving the decision                          | prints the record `survey_branch` returns          |
| Order          | worktree → remote → local (code); worktree → local → remote (man page) | remote → worktree → local (code and man page)      |
| A refusal      | may land after the worktree is gone                                    | lands before anything is touched                   |

**Behaviour, on the cases the fixes came from** (throwaway repository; before = deployed `develop`)

| Case                                 | Before                                                                           | After                                                                    |
| ------------------------------------ | -------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| merged branch, clean linked worktree | closed                                                                           | closed — unchanged                                                       |
| cherry-picked branch, no `--force`   | worktree removed, then `git branch -d` refused; branch survives with no worktree | refused up front; **nothing touched**; message names `--force`           |
| cherry-picked branch, `--force`      | closed (#186)                                                                    | closed — unchanged                                                       |
| dirty worktree, no `--force`         | refused before removal (#181)                                                    | refused; unchanged                                                       |
| dirty worktree, `--force`            | removed, count reported (#181)                                                   | removed, count reported — unchanged                                      |
| remote-only branch                   | remote deleted                                                                   | remote deleted — unchanged                                               |
| remote delete fails                  | worktree already gone; branch alive both sides                                   | **all three traces intact**; one message; re-run when the cause is fixed |
| `--dry-run` of each                  | a separate derivation                                                            | the same record, printed                                                 |

## The record

`survey_branch <branch> <record>` fills an associative array by nameref (bash ≥ 4.3; `declare -A` already
requires ≥ 4.0; Darwin ships 5.3, Git for Windows 5.2):

| Field           | Meaning                                                                                                                                                                               |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `local_branch`  | `refs/heads/<branch>` exists — set independently of the worktree, because `git worktree remove` does not delete the ref                                                               |
| `remote_branch` | origin has the branch                                                                                                                                                                 |
| `worktree_path` | a linked worktree holds it                                                                                                                                                            |
| `dirt_count`    | changed + untracked entries in that worktree, counted _before_ removal — afterwards there is nothing to count                                                                         |
| `branch_flag`   | `delete_flag`'s answer; `-D` under `--force`                                                                                                                                          |
| `refusal`       | why the close will not proceed, or empty. Set when: no local and no remote; `delete_flag` refused and no `--force`; worktree dirty and no `--force`; main-worktree switch was blocked |

`--force` resolves refusals **inside the survey**. A refusal leaves `refusal` set and `remove_branch` does
nothing — "blindly do what is set" is only safe when nothing set is undecided.

`delete_flag`, `forced_cost`, `worktree_dirt`, `worktree_for` are unchanged; the survey calls each once.

## Implementation Phases

### Phase 1: the plan

- [x] This document, the branch's first commit

### Phase 2: `survey_branch` and `remove_branch` — complete

- [x] `survey_branch` produces the record; the only reader of `force`, `delete_flag` and `worktree_dirt`
- [x] `remove_branch` acts on the record: remote, worktree, local — each skipped when unset; stops at the
      first failure and reports which trace failed and what had already been removed
- [x] `preview_line` renders a record as the dry run's line
- [x] **One thing the plan promised and the first cut did not deliver.** Row 2 came back identical to the old
      behaviour: the worktree was removed and then `git branch -d` failed. `delete_flag` answers whether a
      delete is _safe_ and returns `-d`; `git branch -d` then asks whether the branch is _merged_ and refuses
      on its own account. For that refusal to land before anything is touched — goal 2 — the survey has to
      ask git's question too, so it now tests `git merge-base --is-ancestor <branch> <target>` when the flag
      is `-d`. Against the target rather than the upstream, because the remote is deleted first and
      `git branch -d`'s other test is gone by then

### Phase 3: the two loops become one — complete

- [x] The dry-run loop is: survey each, print each, exit
- [x] The live loop is: switch the main worktree if needed (unchanged — it is about the main worktree, not a
      branch's traces), survey, refuse or remove, report. The `blocked` map feeds `refusal`
- [x] The "BOTH halves" comment went with the code it described; the order is stated once in `remove_branch`
      and once in the man page

### Phase 4: the man page — complete

- [x] Steps 4–6: survey and decide; remove in the order remote → worktree → local, with why; report which
      trace failed. The old step 4 promised worktree → local → remote, matching neither the code nor the ruling
- [x] `--force` says it is read once, during the survey; `--dry-run` says the preview is the survey the run
      acts on, and that a refused branch names no removal
- [x] `mandoc -T lint` clean; renders

### Phase 5: acceptance, live — complete

Every row run in a throwaway repository, the deployed `develop` copy beside the new one, each run followed by
a check of all three traces. The remote-failure row is a `pre-receive` hook that rejects the delete, so the
fetch at the top of the run succeeds and only the delete fails — the first fixture re-pointed origin at a
missing path and never reached the delete at all.

| Row                          | OLD (develop)                                                                    | NEW (#204)                                                                                                    |
| ---------------------------- | -------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| merged, clean worktree       | closed; all traces gone                                                          | identical                                                                                                     |
| cherry-picked, no `--force`  | worktree **removed**, then `git branch -d` failed; branch survives with no worktree | `REFUSED: 'git branch -d' will refuse it: its commits are not in 'main'`; **local, remote and worktree untouched** |
| cherry-picked, `--force`     | closed                                                                           | identical                                                                                                     |
| dirty worktree, no `--force` | refused; nothing touched                                                         | identical (wording: "nothing was removed")                                                                    |
| dirty worktree, `--force`    | removed, 3 files reported                                                        | identical; the note now follows the remote delete                                                             |
| remote-only                  | remote deleted                                                                   | identical                                                                                                     |
| **remote delete rejected**   | `worktree=gone` — destroyed — while the message said "nothing was deleted"       | `local=present remote=present worktree=present`; the message is true                                          |
| `--dry-run`, 8 combinations  | —                                                                                | identical to OLD except the cherry case, where the preview now shows the refusal the run makes                 |

- [x] `shell-lint.sh` clean (the record's keys are quoted, so the linter stops reading them as arithmetic);
      `mandoc -T lint` clean
- [ ] **The PR script's own `git close-branch`** — the merged command closing the branch that changed it, on
      the user's PATH, with the deployed copy verified to carry `survey_branch`

## Beyond the table

Three differences the acceptance turned up that the table did not name, all kept:

- **A refused branch's preview no longer says "removes worktree X".** True when a refusal could still strike
  after the worktree was gone; false now that a refusal removes nothing.
- **The failure message's trailing separator is stripped.** The first cut printed `(the worktree was removed; )`.
- **`no local branch, checking the remote` reads `no local branch, closing the remote one`** — the survey has
  already checked by the time it is printed.

## Files to Create/Modify

| File                                                   | Action                           |
| ------------------------------------------------------ | -------------------------------- |
| `docs/plans/chore/204-remove-branch.md`                | Create                           |
| `Home/common/.local/bin/git-close-branch`              | Modify — the refactor            |
| `Home/common/.local/share/man/man1/git-close-branch.1` | Modify — the order and the flags |

## Decisions

- **The main-worktree switch stays outside `remove_branch`.** It is a property of the main worktree, done
  once for every selected branch that lives there; a branch's record only learns whether it was blocked.
- **Behaviour is preserved everywhere the fixes established it** — the six rows marked _unchanged_. The two
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
