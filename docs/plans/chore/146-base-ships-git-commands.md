---
title: "The base layer ships the git-* process commands"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/146
status: in-progress
created: 2026-09-10
updated: 2026-09-10
---

# Plan: the base layer ships the git-* process commands

## Summary

`development-process.md` names `git-open-branch`, `git-close-branch` and `git-reset-branch` as the
reference implementation of an organization-wide process. All three live in the **personal** repository.
The process is owned here; its tools are not. This lands them in the base, beside the `Declare-BashScript`
they source.

This is **this repository's half only** (rescoped 2026-09-08): ops adds the twelve files. Removing them
from personal is `David-Noble-at-work/personal#180`, its own pull request. While both layers carry the
trio, writ resolves the collision by layer — personal wins, the files are identical, nothing observable
changes — until personal#180 removes its copies.

## Goals

1. The base ships the trio, each with its man page and bash and zsh completions, under
   `Home/common/.local/`, beside `Declare-BashScript`.
2. It lints, formats, and renders in its new home with no dependency pointing out of `common`.
3. The process guide names the new location.

## Why `Home/common`, not `Home/noblefactor-ops`

Settled 2026-09-09 (ops#195): a bash script in a *consuming* repository declares its base dependency by
living under `Home/noblefactor-ops[.<selector>]`. The base's own `Declare-BashScript` and `git-*` commands
are the exception — they live in `Home/common`, because Git for Windows brings bash and the `git-*`
commands must be present wherever git is used, whatever layers are selected, and the helper they source
sits beside them in the same `~/.local/bin`, so the dependency stays inside `common`. The rule forbids a
dependency pointing *out* of `common`; this one does not.

## Current State

| | |
| --- | --- |
| Trio, today | `personal/Home/noblefactor-ops/.local/bin/git-{open,close,reset}-branch` (+ man, bash, zsh) |
| `git-close-branch`, today | carries #181, #183, #186 — merged into personal `main` |
| ops `Home/common/.local/bin` | has `Declare-BashScript`; **no** `git-*` yet — a clean target |
| ops shell-lint | walks every file (`find`), resolves the helper with `-P Home/common/.local/bin` |

## What moves — twelve files

`git-{open,close,reset}-branch`, each as four artifacts, from `personal/Home/noblefactor-ops/.local/` into
`noblefactor-ops/Home/common/.local/`:

| Artifact | Path under `.local/` |
| --- | --- |
| script | `bin/<name>` |
| man page | `share/man/man1/<name>.1` |
| bash completion | `share/bash-completion/completions/<name>` |
| zsh completion | `share/zsh/site-functions/_<name>` |

`git-new-workspace` and `git-protect-encrypted-files` do **not** move: ruled 2026-09-09 to reach end of life
and be removed once SOPS is functional, so they stay in personal.

## What does not move, and the coupling that remains

`Declare-BashScript` already ships from this repository's `Home/common/.local/bin` (#147 / #182). So the
trio lands **beside its own source** — the cross-repository `shellcheck -x` problem that blocked this issue
is gone: the sibling resolves directly, and personal's copy is no longer the lint target here.

## Implementation Phases

### Phase 1: the plan
- [x] This document, the branch's first commit

### Phase 2: the twelve files
- [ ] Copy the current trio (with #181/#183/#186) from personal into `Home/common/.local/{bin,share/...}`
- [ ] `chmod +x` the three scripts; the four-artifact layout matches the other base commands
- [ ] Cross-repository, so history cannot follow by `git mv`; the commit message records provenance
      (personal `main` at the fixes). personal#180 does the `git rm` on its side

### Phase 3: it passes in its new home
- [ ] `.github/scripts/shell-lint.sh` clean (shfmt + `shellcheck -x`, helper resolved as sibling)
- [ ] `mandoc -T lint` clean on the three man pages
- [ ] `bash -n` on each script

### Phase 4: the guide names the new location
- [ ] `development-process.md`'s reference-implementation reference points at `Home/common/.local/bin`

## Files to Create/Modify

| File | Action |
| --- | --- |
| `docs/plans/chore/146-base-ships-git-commands.md` | Create — this plan |
| `Home/common/.local/bin/git-{open,close,reset}-branch` | Create (3) |
| `Home/common/.local/share/man/man1/git-{open,close,reset}-branch.1` | Create (3) |
| `Home/common/.local/share/bash-completion/completions/git-{open,close,reset}-branch` | Create (3) |
| `Home/common/.local/share/zsh/site-functions/_git-{open,close,reset}-branch` | Create (3) |
| `docs/guides/development-process.md` | Modify — the reference-implementation location |

## Decisions

- **Copy at the current tip, not an older revision.** personal's trio now carries #181/#183/#186; the base
  should ship the fixed command, not a version that predates them.
- **`git mv` is the personal side's tool, not this one.** History cannot cross repositories; provenance is
  recorded in the commit message instead. personal#180 uses `git rm`.
- **The trio's record follows in later PRs, not this one.** The plans and issues for the trio belong here
  too (ruled 2026-09-09), but a pull request carries files; moving issues (`gh issue transfer`) and copying
  plan documents are separate, and #180 plus a documentation pass are where they land. This PR is the twelve
  files and the one guide reference.
- **#185 (`remove_branch`) waits for this.** The refactor lands in the trio's new home, so the seam is fixed
  once, here, not in personal and then moved.

## Related Documents

- Issue #146; feature #158; personal#180 (the removal half); ops#195 (the placement rule that unblocked the
  destination); #147/#182 (Declare-BashScript already shipping from base); #185 (the refactor that follows)

## Open Questions

None.
