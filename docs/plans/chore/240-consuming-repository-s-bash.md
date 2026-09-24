---
title: "A consuming repository's bash scripts live in common, beside the Declare-BashScript they source"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/240
status: complete
created: 2026-09-24
updated: 2026-09-24
---

# Plan: a consuming repository's bash scripts live in common

## Issue 240

`development-process.md` §"Where a script lives" requires a bash script in a consuming repository to
live under `Home/noblefactor-ops[.<selector>]` rather than `common*` (#147, #195). `Declare-BashScript`
ships in this repository's `Home/common/.local/bin`, and the base is registered wherever a consuming
repository deploys, so a script in a consuming repository's `Home/common[.<selector>]` sources a
sibling in the same `common` and nothing points out of it. Ruled 2026-09-24.

Found while moving personal's `Upgrade-Debian` to `common.Linux.Debian`. The global CLAUDE.md repeats
the stale rule; a personal issue rewrites it to reference this document instead, after this merges.

## Goals

1. §"Where a script lives" states the corrected rule.
2. The text that exists only to justify `noblefactor-ops[.<selector>]` for consuming repositories goes.
3. Nothing else changes.

## Requirements

### Requirement 1: The section

`docs/guides/development-process.md` §"Where a script lives", from its heading to the lint paragraph,
becomes:

> ## Where a script lives
>
> The rule, as ruled 2026-09-07 (noblefactor-ops#147) and corrected 2026-09-24 (noblefactor-ops#240):
>
> - Code that takes a dependency on, or contributes to, a repository's stack goes into a directory named
>   for that repository: `Home/noblefactor-ops[.<selector>]` for anything that needs the base beyond
>   `Declare-BashScript`.
> - If not, it goes elsewhere. What is to be deployed unconditionally, wherever you go, goes into
>   `common[.<selector>]` — so `common` depends on nothing outside itself.
> - Every bash script sources `Declare-BashScript`. It is what gives a script `--help`, a man page and
>   exit codes; a script without it is below spec, not portable. `Declare-BashScript` ships in this
>   repository's `common`, and the base is registered wherever a consuming repository deploys, so a bash
>   script in any repository's `common[.<selector>]` sources a sibling in the same `common`: nothing
>   points out of it.
> - **The `git-*` commands** live beside `Declare-BashScript` in this repository's `common` (ruled
>   2026-09-09). Git for Windows brings bash, so they must be present wherever git is used, whatever
>   layers are selected.
> - **A git hook** sources nothing (ruled 2026-09-08): git runs it from the hooks path, where there is
>   no sibling to source, and a hook is configuration git carries rather than a command anyone runs.
> - **A context project keeps its scripts, and they source the base's file where they are.** Ruled
>   2026-09-08. A project named for nothing configured — a family's, an employer's — is deployed by name
>   on top of the whole stack; the project name keeps its own meaning, which is who the scripts are for
>   (personal#177).

Removed: the paragraph beginning "writ makes the first bullet enforceable" and its consequence bullet
"In a consuming layer, the selector says what the script needs". The lint paragraph that follows is
unchanged.

### Requirement 2: The document's own metadata

`updated:` becomes 2026-09-24.

## Implementation Phases

### Phase 1: Commit the plan

- [x] This plan, before the change

### Phase 2: The section

- [x] §"Where a script lives" replaced (Requirement 1)
- [x] Frontmatter `updated` (Requirement 2)
- [x] No other non-plan document states the old rule — `git grep` outside `docs/plans` finds
      `development-process.md` alone

### Phase 3: Verify, then merge

- [x] The document gates pass on this host — frontmatter 48 checked, 0 errors; codespell 2.4.3 with
      CI's arguments, clean. Shell, PowerShell and Starlark cover no file this change touches and run in CI
- [x] PR script written, shown, and handed over

## Out of Scope

**The ruling plans #146, #147 and #195.** They record what was ruled then.

**personal's CLAUDE.md and its `noblefactor-ops.*` layer.** Separate personal issues, after this merges.

## Related Documents

- [#147](https://github.com/NobleFactor/noblefactor-ops/issues/147), [#195](https://github.com/NobleFactor/noblefactor-ops/issues/195) — the rulings corrected
- [development-process.md § Where a script lives](../../guides/development-process.md#where-a-script-lives)
