---
title: "PR Script Template: pre-flight with the command CI actually runs"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/131
status: in-progress
created: 2026-08-30
updated: 2026-08-30
---

# Plan: PR script CI pre-flight

## Summary

The PR script template has no verification step between staging and committing. A generated script
therefore pushes whatever the author believed was clean, and the first real check happens in CI —
after the branch, the issue, the commit and the PR already exist. This plan adds one step to the
template: run the repository's own CI entry point before committing, and refuse to proceed if it
fails.

## Motivation

Observed 2026-08-30 in the `personal` repository. A PR was prepared with a hand-rolled pre-flight —
`shellcheck -x` on the changed scripts, `mandoc -T lint` on the changed man page. Both passed. CI
then failed:

```
✗ ./Home/noblefactor.Unix/.local/bin/Start-Claude
  shfmt: FAIL
```

The repository's gate is `shfmt -i 4 -ci` **and** `shellcheck -x --severity=warning`. The
pre-flight checked one of the two. Nothing was wrong with the reasoning — it was a plausible subset
— and that is the point: a subset chosen from memory will differ from the gate eventually, and the
cost is a failed PR that already has an issue and a branch attached to it.

The fix is not a longer checklist. It is to stop maintaining a second copy of the gate.

## Goals

1. **The pre-flight is the gate, not a model of it.** Scripts invoke the repository's own entry
   point, so passing locally and passing in CI are the same check by construction.
2. **Stay correct across repositories.** The entry point differs per repository; the template must
   not name one.
3. **Fail before side effects.** The check runs before the issue is created and before the commit.

## Current State

| Step in the template | Status |
| --- | --- |
| Stage specific files | present |
| Commit | present |
| Push, create PR | present |
| CI gate (`gh pr checks --watch`) | present |
| Merge gate, squash, `git close-branch` | present |
| **Verify anything before committing** | **absent** |

The CI gate catches a bad push, but only after the branch, issue, commit and PR exist. Recovering
means a follow-up commit on an open PR with a red check.

## Requirements

### Requirement 1: The template gains a pre-flight step

Placed after staging and before the tracking issue is created, so a failure costs nothing:

```bash
# --- Pre-flight: the repository's own gate, not an approximation of it ---
#
# Run whatever CI runs. Passing here must mean passing there, which is only true if it is
# literally the same command.

.github/scripts/shell-lint.sh
```

`set -euo pipefail` at the top of every generated script means a non-zero exit aborts. No `if`
wrapper is needed and none should be added, since a swallowed failure defeats the step.

### Requirement 2: The command is discovered, not remembered

The template must not hard-code `shell-lint.sh`. It is correct for `personal` and `devlore-cli` and
wrong for `noblefactor-ops`, which has no `.github/scripts` at all. The instruction is to read the
workflow and use what it invokes:

| Repository | CI entry point |
| --- | --- |
| `personal` | `.github/scripts/shell-lint.sh` |
| `devlore-cli` | `make vet-all`, `make lint-all`, `./build/star lint go ./...` |
| `noblefactor-ops` | `./.github/scripts/Test-Frontmatter.sh`, `codespell` |

Where a repository routes through a build tool, the pre-flight calls that target rather than the
underlying binary — the target is what CI runs, and it carries the flags.

### Requirement 3: Scope follows the entry point

Where the entry point walks the tree, the pre-flight walks the tree — a change can break a file it
does not touch, and CI judges the whole tree regardless. Where it takes paths, pass the paths CI
passes. Where CI includes a long build or test matrix, the pre-flight covers the lint and vet
stages; that is where formatting failures live, and a step believed to be slow is a step that gets
dropped.

### Requirement 4: The table is an example, not a source of truth

Read the workflow each time. This was demonstrated during the change itself: `noblefactor-ops`
removed its Go trees and re-based its gate on `Test-Frontmatter.sh` plus `codespell` in commit
`87d77cc`, landing between this plan being written and its PR script being run. A pre-flight of
`go build ./...` would have run against a repository with zero `.go` files.

Where CI uses a GitHub Action rather than a command, run the CLI it wraps with the workflow's
arguments — the one place the local command is not literally identical, and worth naming so it is
not mistaken for licence to invent a subset.

## Implementation Phases

### Phase 1: Add the step to the template (status: complete)

Insert the pre-flight block into `docs/guides/pr-script-template.md` between staging and commit,
with the per-repository table and the discovery instruction.

### Phase 2: Verify (status: complete)

- The template's example block is valid bash under `set -euo pipefail`
- The three entry points in the table are the commands those workflows actually invoke
- `Test-GuideFrontmatter.sh` equivalent, if this repository gates guide frontmatter

## Verification

| Check | Result |
| --- | --- |
| Template's bash block parses under `bash -n` | valid, 84 lines |
| Pre-flight precedes the commit | line 11, commit at line 20 |
| Entry points in the table match the workflows | confirmed against all three repositories |
| `CLAUDE.md` needs no companion change | step 3 already says to follow this template |

## Out of Scope

- Adding `.github/scripts/shell-lint.sh` to `noblefactor-ops`. Its CI is Go, and a shell linter
  there is a separate question.
- Retrofitting existing `go-*` scripts on disk. They are one-offs for merged branches.
- Pre-commit hooks. The house rules already forbid bypassing hooks; this plan is about the
  generated script, which runs before any hook would.

## Resolved

- **How much of CI to run** — settled by Requirement 3. Lint and vet, not a full build matrix.
- **Whether to show a per-linter fallback with explicit flags** — no. Copying flags into the
  template recreates the second copy of the gate that this plan exists to remove.
- **Whether `CLAUDE.md` needs a matching note** — no. Step 3 of the PR Process already says to
  follow this template, so the step arrives with it. `CLAUDE.md` is also tracked in the `personal`
  repository, so a note there would make this a two-repository change for no gain.
