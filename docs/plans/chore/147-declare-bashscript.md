---
title: "Declare-BashScript ships from the base layer, and every script that sources it says so by project"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/147
status: in-progress
created: 2026-09-06
updated: 2026-09-06
---

# Plan: Declare-BashScript ships from the base layer, and every script that sources it says so by project

## Summary

Sixty-one scripts in `personal`, one in `devlore-cli` and the three `git-*` commands waiting in #146's
worktree source `Declare-BashScript`; only `personal` contains it. The first draft of this plan left
every script where it was and injected the file's path at lint time from a lock. Three rulings on
2026-09-06 replaced that: the file ships from the base layer's `common`; a script declares what it
depends on by the project it lives in, `noblefactor-ops[.<selector>]`, which devlore-cli#850 makes an
implicit project wherever the base is configured; and the base is public (#179), so lint follows the
file at a pinned ref with nothing secret. This plan is three repositories in dependency order, each
its own PR cycle, and it names every file that moves.

## Goals

1. **One copy, in the base's `common`**, with its man page and its bash and zsh completions; gone
   from `personal`, bridge symlink included.
2. **Every consumer's project says what it depends on.** In `personal`, `noblefactor-ops.<selector>`
   for Unix-only bash, `noblefactor-ops` with no selector for the two git-supporting scripts; in
   `devlore-cli`, `Home/noblefactor-ops.Unix`. `common` in every layer depends on nothing outside itself.
3. **Lint follows the file it runs against**: locally the deployed one, in CI the public base at a
   pinned ref. Following it is stricter than not, and that is the point.
4. **Runtime unchanged.** writ merges every layer into one `~/.local/bin`; `$(dirname "$0")/Declare-BashScript`
   resolves before and after.

## The rule this plan applies

Layers are repositories, and a repository's name is a project name: `noblefactor-ops` for the base,
`devlore-cli` for the team layer in the worked example, `personal` for the personal layer.
`common[.<selector>]` is what a repository gives every consumer unconditionally, so it may depend on
nothing outside its own `common`. Content that depends on a layer says so by living in a project
named for that layer, keeping its platform selector. devlore-cli#850 makes those projects implicit —
a bare `writ deploy` brings `Home/noblefactor-ops{,.Unix,.Darwin}` from every layer that has it — and
#849 is the design record. A context project such as `thenobles` is named on the command line and, by
this plan, depends on nothing outside itself either.

## Measured, 2026-09-04 to 2026-09-06

| Fact | Measurement |
| --- | --- |
| Consumers in `personal` | 61: `common.Darwin` 21, `common.Unix` 10 (one in the secrets tree), `noblefactor.Unix` 9, `common.Linux` 8, `common` 3, `thenobles.Darwin` 3, `noblefactor` 3, `microsoft.Unix` 3, `common.Debian` 1 |
| Other bash that moves with them | `Get-JavaInfo` (`common.Darwin`, sources nothing); `install` and `templates/` rename with `noblefactor.Unix` |
| Consumers elsewhere | `devlore-cli/scripts/New-DevloreKeyVaults` (PATH form, `source "Declare-BashScript"`); the three `git-*` in #146's worktree |
| The file in `personal` | one file, `Home/common/.local/bin/Declare-BashScript`, blob `e38ff347`, last changed `aa067d83` 2026-08-20; `Home/common/local/bin/Declare-BashScript` is a symlink to it (mode `120000`), the bridge the 2026-08 migration left |
| Companion assets | man page, bash completion, zsh function under `Home/common/.local/share/`; 123 such files travel with the 52 scripts that move project |
| The no-dot tree | 45 consumers under `*/local/bin`; `noblefactor*` has `.local/bin` only; both `~/local/bin` and `~/.local/bin` are on PATH; the 2026-08 plan says "eventually all bash scripts move to `.local/bin` and the bridge is retired" |
| Directives | 10 repo-relative `# shellcheck source=Home/…`; the bare form resolves through `-P`, the repo-relative form does not; following the source adds one finding, `az-ssh` SC2154 |
| Gates | `personal`: `shell-lint.sh` (`shfmt -d -i 4 -ci`, `shellcheck -x --severity=warning`) from `quality-gate.yaml`; `noblefactor-ops`: no shell gate, and its four bash scripts plus `Declare-BashScript` pass personal's today; `devlore-cli`: `star lint shell`, no source-path option |
| The base | public since 2026-09-06; the raw `LICENSE` URL answers 200 without a token |
| The team layer | `devlore-cli/Home/common/packages-manifest.yaml` is its whole `Home` today |
| What the six context scripts use | `require_darwin` ×3, `require_nix` ×3, `error` ×2, `success` ×1, `EX_*` ×2, and `Backup-TimeCapsule` alone uses `usage` and the argument parsing |
| While both layers carry the file | writ resolves a collision by layer, then specificity: `personal` wins, and the two are identical |

## Implementation Phases

### Phase 1: `noblefactor-ops` — the file, its assets, the gate, the rule

- [x] `Home/common/.local/bin/Declare-BashScript`, `personal`'s blob `e38ff347` with two changes: the
      header is Apache-2.0, as everything here is since #180, and the line that tells consumers how to
      source it says the bare directive form
- [x] `Home/common/.local/share/man/man1/Declare-BashScript.1`,
      `Home/common/.local/share/bash-completion/completions/Declare-BashScript`,
      `Home/common/.local/share/zsh/site-functions/_Declare-BashScript`
- [x] `.github/scripts/shell-lint.sh` — `personal`'s, with
      `-P "${DECLARE_BASHSCRIPT_DIR:-Home/common/.local/bin}"`; a `Shell` step in `ci.yaml` installing
      shellcheck from apt and shfmt `v3.14.0` pinned by digest (`fe42021c…0b66`, computed from the
      release asset: upstream publishes no checksum file)
- [x] `docs/guides/development-process.md` gains "Where a script lives": the rule above in one
      paragraph, with the two exceptions named (git-supporting → `common`; context projects carry no
      dependency)
- [x] `docs/guides/pr-script-template.md`'s CI table: the `noblefactor-ops` row gains the shell gate;
      the `personal` row names `DECLARE_BASHSCRIPT_DIR`
- [ ] **Acceptance:** the ops gate is green with the file in-tree; after `writ deploy` here and on
      DANOBLE-WD11-3, `readlink ~/.local/bin/Declare-BashScript` still names `personal`'s copy —
      precedence — and nothing observable changes yet

**Files**: `Home/common/.local/bin/Declare-BashScript`, three assets, `.github/scripts/shell-lint.sh` — Create;
`.github/workflows/ci.yaml`, `docs/guides/development-process.md`, `docs/guides/pr-script-template.md` — Modify

### Phase 2: `personal` — rename, move, declare, inline, delete

Its own issue and plan there (`docs/plans/chore/<n>-declare-bashscript-by-project.md`, labels `chore`,
`Epic:Process`). One PR; `git mv` throughout so history follows.

- [ ] `Home/noblefactor` → `Home/noblefactor-ops`; `Home/noblefactor.Unix` → `Home/noblefactor-ops.Unix`
      (12 scripts, `install`, `templates/`, assets); `install`'s two header URLs follow
- [ ] The 44 `common*` consumers and their assets, into `.local/{bin,share/…}`: `common.Darwin` 21 +
      `Get-JavaInfo` → `noblefactor-ops.Darwin`; `common.Linux` 8 → `noblefactor-ops.Linux`;
      `common.Debian` 1 → `noblefactor-ops.Debian`; `common.Unix`'s 9 + `ConvertTo-Pdf` →
      `noblefactor-ops.Unix`; `git-new-workspace`, `git-protect-encrypted-files` → `noblefactor-ops`
- [ ] Every mover carries the bare directive; the 10 repo-relative ones are rewritten; `az-ssh`'s
      SC2154 fixed
- [ ] The six context scripts stop sourcing it: `require_darwin`/`require_nix`/`error`/`success`
      inlined where used; `Backup-TimeCapsule` gets its own `getopt` and usage text
- [ ] `Home/common/.local/bin/Declare-BashScript`, the bridge symlink, and the three assets deleted
- [ ] `Home/common/.claude/CLAUDE.md` lines 120–121 — the stale `dotfiles/Configs` guidance — replaced
      by the rule
- [ ] `quality-gate.yaml`: `actions/checkout` of `NobleFactor/noblefactor-ops` at a pinned `ref` into
      `.base/`, `DECLARE_BASHSCRIPT_DIR` exported; `shell-lint.sh` prunes `.base` and passes
      `-P "${DECLARE_BASHSCRIPT_DIR:-$HOME/.local/bin}"`. The ref is the pin; moving it is a visible commit
- [ ] **Acceptance:** CI green with the file absent from the repository; locally, lint follows the
      deployed file; after `writ deploy`, `readlink ~/.local/bin/Declare-BashScript` names the base,
      every moved command answers on PATH, `git close-branch --help` renders; `Fixup-MusicLibrary.applescript`
      and the Windows files are the only things left under `common*/local/bin`

### Phase 3: `devlore-cli` — #476, the one script into its own `Home`

- [ ] `git mv scripts/New-DevloreKeyVaults Home/noblefactor-ops.Unix/.local/bin/New-DevloreKeyVaults`;
      its PATH-form `source "Declare-BashScript"` becomes the sibling form with the bare directive
- [ ] `star lint shell` follows nothing and passes as today; recorded on devlore-cli#721 as the
      source-path option the built-in wants
- [ ] **Acceptance:** devlore-cli's gates green; after `writ deploy`, `~/.local/bin/New-DevloreKeyVaults`
      is the team layer's; #476 closes

### Phase 4: the record

- [ ] #146's "What does not move" paragraph is corrected and the issue is unblocked
- [ ] #150 inherits the rename: `Start-Claude` is `noblefactor-ops.Unix` in `personal` until its own
      move; its issue says so
- [ ] DANOBLE-WD11-3: base and personal pulled and deployed after Phase 2, before anything relies on it

## Decisions

- **Scripts stay in their layers; the project declares the dependency.** Ruled 2026-09-06, after two
  earlier readings the same day — inject-and-pin, then move-everything-into-ops — were each measured
  and set aside. This one dissolves the cross-repository lint problem instead of mechanizing it.
- **`common` depends on nothing outside itself.** Git-supporting scripts are `common` in the base
  because Git for Windows brings bash; in a consuming layer a git-supporting script that sources
  `Declare-BashScript` is `noblefactor-ops` with no selector.
- **The selector survives the rename.** A Darwin-only script stays off Linux machines; `.Unix` is the
  default the ruling named, not a flattening.
- **`.local/bin` for what moves; the no-dot tree's retirement stays separate.** The 2026-08 plan's
  intent, applied to the 45 this touches; the AppleScript and the Windows files are not this plan's.
- **Context projects drop the dependency rather than bend the rule.** Five scripts lose one function
  call each; `Backup-TimeCapsule` regains its own argument parsing. The alternative — a `thenobles`
  script signalling `noblefactor-ops` — loses the family grouping to save a dozen lines.
- **CI follows the public base at a pinned ref.** No token, no lock file: the `ref:` in the workflow
  is the pin, and git guarantees the content at a ref. The first draft's digest lock solved a problem
  the public repository no longer has.
- **The bridge symlink stays until it stops being load-bearing, and goes then.** Ruled 2026-09-06: "keep
  it and get rid of it as soon as it is practical." Practical is Phase 2's PR — 33 deployed scripts in
  `~/local/bin` and 45 in the repository resolve through it today, and the move into `.local/bin` is
  what ends that. It is deleted in the same commit as the move, not before.
- **The file leaves `personal` in the same PR that renames the projects.** After Phase 1's deploy the
  base already carries it; while both do, `personal`'s wins and they are identical.
- **`New-DevloreKeyVaults` stays in devlore-cli**, in `Home/noblefactor-ops.Unix`. devlore-cli is a
  layer with a `Home` of its own, and content that depends on the base belongs in *its* repository-named
  project, not the base's. Revised from the 2026-09-06 echo, which assumed devlore-cli had no `Home`.
- **`install` renames with its project.** Only its own header cites the raw URL.
- **`Import-GnuPg` is untouched.** It sources a file that is not beside it and uses nothing from it; a
  defect of its own, in the secrets tree, not this plan's.

## Related Documents

- Issue #147 — this chore; feature #158, epic #142; `Thread:WritOrigin`, `Thread:Ops:PortableTooling`
- #146 — the `git-*` commands, resuming after Phase 1; #150 — `Start-Claude`; #179 — the base went public
- NobleFactor/devlore-cli#850 — implicit projects; #849 — the writ layer design record; #476 — Phase 3's issue;
  #721 — the built-in linter, and `star lint shell`'s missing source-path option
- `personal` — `docs/plans/migrate-declare-bashscript-to-dot-local.md`, the 2026-08 migration whose
  bridge this plan removes

## Open Questions

- [ ] **devlore-cli's own `Home/noblefactor-ops.Unix` for `New-DevloreKeyVaults`**, rather than the
      base's. This is the one placement that changed after the echo you confirmed, because devlore-cli
      turned out to be a layer with a `Home`. Proposed: devlore-cli's.
