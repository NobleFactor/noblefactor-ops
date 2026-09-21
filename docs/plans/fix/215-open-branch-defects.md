---
title: "git-open-branch maps the chore label to chore/"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/215
status: complete
created: 2026-09-21
updated: 2026-09-21
---

# Plan: git-open-branch maps the chore label to chore/

## Issue 215

Lane 3 of #217. `branch_type_for` knew `bug`, `documentation`, `enhancement`, `feature` and `epic`,
so an issue labelled `chore` fell to the `feature` default: `git open-branch 191` opened
`feature/191-...` and had to be closed and reopened with `-T chore` by hand. The ruleset allows
`chore/`, both repositories carry the label, and the plan path follows the prefix, so the plan
misfiled too.

Ruled 2026-09-21: fix the chore label, nothing else.

## Requirements

### Requirement 1: `chore` maps to `chore`

One arm in `branch_type_for`, ahead of `documentation`, and the matching row in the man page's
mapping table.

## Implementation Phases

### Phase 1: The change

- [x] `*,chore,*) printf 'chore' ;;` in `Home/common/.local/bin/git-open-branch`
- [x] `chore;chore` in the table in `Home/common/.local/share/man/man1/git-open-branch.1`

### Phase 2: Verify

- [x] `bash -n` passes
- [x] The worktree's script, `--dry-run`: chore issue #216 opens `chore/216-...`; bug issue #214 still
      opens `fix/214-...` -- 2026-09-21

## Files

| File | Action | Purpose |
| --- | --- | --- |
| `Home/common/.local/bin/git-open-branch` | Modify | The mapping |
| `Home/common/.local/share/man/man1/git-open-branch.1` | Modify | The documented mapping |

## Out of Scope

The rest of what #215's investigation raised -- `task`, `epic`, the `documentation` and `enhancement`
arms, refusing an issue with no kind label -- is not ruled and not in this change.
