---
title: "git open-branch leaves the upstream unset"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/247
status: active
created: 2026-09-26
updated: 2026-09-26
---

# Plan: git open-branch leaves the upstream unset

## Summary

`git open-branch` creates a branch from `origin/<target>`, which makes that ref the branch's upstream.
A bare `git push` then refuses, because the upstream's name differs from the branch's. Adding
`--no-track` to both creation paths leaves the slot empty, so a consumer's `push.autoSetupRemote`
creates the correct upstream on the first push and `git push` works.

## Goals

1. **A branch this tool opens pushes with a bare `git push`** — no `--set-upstream`, no
   `git branch --unset-upstream` first.
2. **The tool is correct on a machine that has not configured around the problem** — no dependency on
   a consumer setting `branch.autoSetupMerge`.
3. **The note the tool prints says what now works**, rather than the workaround it used to need.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| `git checkout -b` path, line 235 | Broken | upstream becomes `origin/<target>` |
| `git worktree add -b` path, line 244 | Broken | same |
| The note at line 275 | Documents the bug | tells the operator to use `-u`, hours before they push |
| `push.autoSetupRemote` in consumers | Set, and inert | its condition is an empty upstream slot |

Measured 2026-09-26 on DANOBLE-WD11-3, on a branch this tool had just opened:

```
branch.fix/230-plaintext-key-outside-git-crypt.remote = origin
branch.fix/230-plaintext-key-outside-git-crypt.merge  = refs/heads/main
```

`git push` refused with `The upstream branch of your current branch does not match the name of your
current branch`.

**This branch reproduced it while being created for this plan:**

```
branch 'fix/247-open-branch-no-track' set up to track 'origin/develop'.
```

## Requirements

### Requirement 1: both creation paths pass `--no-track`

The script creates a branch two ways, and both take a remote-tracking ref as the start point:

```bash
git checkout -b "${branch}" "origin/${target_branch}"                                  # no worktree
git worktree add --no-checkout "${worktree}" -b "${branch}" "origin/${target_branch}"  # worktree
```

`branch.autoSetupMerge` defaults to `true`, so each sets upstream to `origin/<target>`.
`push.autoSetupRemote`, which git 2.37 added to solve this, only acts when a branch has **no** upstream
— so it never fires. The ordering is the whole bug: one setting acts at creation, the other only at push
and only into an empty slot.

`--no-track` on both leaves `branch.<name>.remote` and `branch.<name>.merge` unwritten.

**Verified 2026-09-26** with a scratch branch:

| | `branch.remote` | `branch.merge` |
| --- | --- | --- |
| `git branch --no-track zz origin/main` | unset | unset |
| the branch this tool opened | `origin` | `refs/heads/main` |

`git push --dry-run` on the `--no-track` branch reported
`* [new branch] zz-upstream-proof -> zz-upstream-proof`.

### Requirement 2: the note tells the truth

Line 275 currently explains the defect and prescribes the workaround:

```bash
note "First push: git -C '${worktree}' push -u origin '${branch}'"
```

It becomes `Push with: git -C '${worktree}' push`.

### Requirement 3: `branch.autoSetupMerge` is not touched

Setting it to `simple` in a consumer's `config.common` would fix every branch-creation path rather than
this one tool. It is also a per-machine configuration change in another repository, and this tool must
be correct on a machine that has not made it. Out of scope here; noted in the issue.

## Implementation Phases

### Phase 1: the plan lands

- [x] This document is reviewed and approved -- 2026-09-27
- [x] Committed before any other change on this branch

### Phase 2: the fix

- [ ] `--no-track` on the `git checkout -b` path
- [ ] `--no-track` on the `git worktree add` path
- [ ] The note at line 275 says `git push`
- [ ] `bash -n` parses; shellcheck passes in CI

**Files**: `Home/common/.local/bin/git-open-branch` — modify.

### Phase 3: proof

- [ ] This branch's own upstream is cleared and a bare `git push` works
- [ ] After the merge and `writ deploy common`, a throwaway branch opened by the deployed tool has
      `branch.remote` and `branch.merge` unset, and `git push --dry-run` names the branch after itself
- [ ] The throwaway branch is deleted

### Phase 4: handover

- [ ] `go-noblefactor-ops.ps1` written to the PR script template's Windows form
- [ ] Shown with a summary, and the one command handed over
- [ ] The owner runs it

### Phase 5: after the merge

- [ ] `writ deploy common`
- [ ] This document set to `complete` in the last commit

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `Home/common/.local/bin/git-open-branch` | Modify | `--no-track` on both paths; correct the note |
| `docs/plans/fix/247-open-branch-no-track.md` | Create | This plan |

## Related Documents

- Issue #247 — the defect, with the measurement and the two-defaults explanation
- `git-config(1)` — `branch.autoSetupMerge`, `push.autoSetupRemote`, `push.default`

## Open Questions

None. The scratch-branch measurement settled the mechanism, and the scope question — whether to fix the
tool or the consumer configuration — is answered in Requirement 3.
