---
title: "The base layer ships the git-* process commands"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/146
status: complete
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

### Phase 2: the twelve files — complete
- [x] Copied the current trio from personal `main` @ `c153d90e` (carries #181/#183/#186) into
      `Home/common/.local/{bin,share/...}`
- [x] `chmod +x` the three scripts; the four-artifact layout matches the other base commands
- [x] Cross-repository, so history cannot follow by `git mv`; provenance is in the commit message.
      personal#180 does the `git rm` on its side
- [x] Relicensed all twelve to this repository's convention — see Decisions

### Phase 3: it passes in its new home — complete
- [x] `.github/scripts/shell-lint.sh` clean on all six shell files (shfmt + `shellcheck -x`, the helper
      resolved as a sibling in `Home/common/.local/bin` — the cross-repository problem that blocked this
      issue is gone)
- [x] `mandoc -T lint` clean on the three man pages, after the header and COPYRIGHT edits; renders
- [x] `bash -n` on each script
- [x] Repo-wide `codespell` with CI's flags clean; `Test-Frontmatter` 46 checked, 0 errors; buildifier clean
- [x] No `MIT` residue anywhere in the twelve — headers *and* the man pages' closing license sections

### Phase 4: the guide names the new location — complete
- [x] `development-process.md`'s "Reference implementation" section stated **no** location, so this is an
      addition, not a replacement: one paragraph naming `Home/common/.local/bin`, the sibling
      `Declare-BashScript`, the four artifacts, and why it deploys wherever git does

## Files to Create/Modify

| File | Action |
| --- | --- |
| `docs/plans/chore/146-base-ships-git-commands.md` | Create — this plan |
| `Home/common/.local/bin/git-{open,close,reset}-branch` | Create (3) |
| `Home/common/.local/share/man/man1/git-{open,close,reset}-branch.1` | Create (3) |
| `Home/common/.local/share/bash-completion/completions/git-{open,close,reset}-branch` | Create (3) |
| `Home/common/.local/share/zsh/site-functions/_git-{open,close,reset}-branch` | Create (3) |
| `docs/guides/development-process.md` | Modify — the reference-implementation location |
| `.github/codespell-ignore` | Modify — three words, with reasons |

## Decisions

- **Copy at the current tip, not an older revision.** personal's trio now carries #181/#183/#186; the base
  should ship the fixed command, not a version that predates them.
- **`git mv` is the personal side's tool, not this one.** History cannot cross repositories; provenance is
  recorded in the commit message instead. personal#180 uses `git rm`.
- **The trio's record follows in later PRs, not this one.** The plans and issues for the trio belong here
  too (ruled 2026-09-09), but a pull request carries files; moving issues (`gh issue transfer`) and copying
  plan documents are separate, and #180 plus a documentation pass are where they land. This PR is the twelve
  files and the one guide reference.
- **The twelve are relicensed Apache-2.0 on arrival.** They carried `SPDX-License-Identifier: MIT` and an
  MIT redistribution sentence from personal. #179 relicensed this repository to Apache-2.0; landing
  MIT-headed files in it would make it mixed-license. Headers now match the sibling `Declare-BashScript`
  exactly (`Apache-2.0`, `2025-2026`, no redistribution line). Content is otherwise a copy.
- **Three codespell words are ignored, with reasons, not skipped by file.** `remainin` is `git-open-branch`'s
  own example of a name truncated mid-word — correcting it destroys the illustration, the precedent already
  recorded for `runn`. `te` and `bu` are roff macros (`.TE`, `.IP \(bu`) codespell tokenizes as words; ops
  had not met them because its one prior man page uses neither. Skipping `*.1` would hide real man-page
  typos, so the words are listed instead. One genuine respelling in `git-open-branch`: `preempting`, from its British hyphenated form.
- **#185 (`remove_branch`) waits for this.** The refactor lands in the trio's new home, so the seam is fixed
  once, here, not in personal and then moved.

## Related Documents

- Issue #146; feature #158; personal#180 (the removal half); ops#195 (the placement rule that unblocked the
  destination); #147/#182 (Declare-BashScript already shipping from base); #185 (the refactor that follows)

## Open Questions

None.
