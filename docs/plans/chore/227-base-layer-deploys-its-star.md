---
title: "The base layer deploys its star extension under devlore/"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/227
status: complete
created: 2026-09-23
updated: 2026-09-23
---

# Plan: The base layer deploys its star extension under devlore/

## Issue 227

Lane 15 of #217, second half. [devlore-cli#918](https://github.com/NobleFactor/devlore-cli/issues/918)
moved star's data under `devlore/`, the way writ's already was, and shipped as
[devlore-cli#921](https://github.com/NobleFactor/devlore-cli/pull/921) on 2026-09-23. star now probes
`~/.local/share/devlore/star/extensions` first and the old `~/.local/share/star/extensions` second,
the second only so nothing breaks before each consumer redeploys.
[devlore-cli#920](https://github.com/NobleFactor/devlore-cli/issues/920) removes the old probes.

This repository is such a consumer, and the one that must move first: it deploys
`com.noblefactor.ops.GitHub`, the extension behind `star gh issues report|audit` and
`star gh labels audit|sync` — the commands this repository's own process runs.

The audit that preceded this plan found more star material here than the deployment: seven
architecture documents, eight star-era plans, two guides, a JSON schema and a working Rust/WASM
example. Six of the seven architecture documents are out of scope and filed separately (see **Out of
scope**). One is not, because the audit found nothing anywhere that it describes.

## Goals

1. The base layer deploys its extension where star now looks for it, and the README says so.
2. `star-wasm-receivers.md` goes, in a commit of its own, with the reasoning in the message.
3. The live path is verified on this box in the right order, which is not the obvious one.

## Requirements

### Requirement 1: The deployment moves

```
Home/common/.local/share/star/extensions/com.noblefactor.ops.GitHub/
  → Home/common/.local/share/devlore/star/extensions/com.noblefactor.ops.GitHub/
```

Six files — `extension.yaml` and `commands/{gh-issues-audit,gh-issues-report,gh-labels-audit,gh-labels-sync,scheme}.star`
— moved with `git mv`, contents unchanged. `README.md:34` names the old path in the sentence
describing what the layer ships; it names the new one.

### Requirement 2: The WASM receiver document goes

`docs/architecture/star-wasm-receivers.md` (218 lines) specifies a shared-memory reactor protocol
for WASM extension modules under wazero. It describes a subsystem that exists in no repository:
devlore-cli has no `wazero` in `go.mod`, no `.wasm` files and no `internal/wasm/`, and its
`docs/plans/move-star-to-devlore-cli.md:35` still records `internal/wasm/` as "TBD — not yet moved".

This repository had one WASM extension, `com.noblefactor.star.Gitignore`, and
`docs/architecture/star-file-tree-walking.md` exists to delete it in favour of native Go over
go-git. That shipped — `pkg/gitignore/tracker.go` is live in devlore-cli.

Nothing references the document: no link in this repository, no entry in
`.github/frontmatter-exempt` (it carries proper frontmatter), and devlore-cli's
`docs/architecture/index.md` §9 lists "Star WASM Receivers … (planned)" with no target file.

A separate commit, so the reasoning is legible on its own.

### Requirement 3: The live path, in order

star must be rebuilt from devlore-cli `develop` **before** the deploy. The binary installed on this
box is build `20260922122246`, older than #921, so it knows only the old probe and would stop
finding the extension the moment it moves. The order is: rebuild, then pull writ's base clone
(`~/.local/share/devlore/writ/repos/noblefactor-ops`, a clone distinct from the working one), then
`writ deploy common`.

"Resolves" means the command is found. `star gh issues report` still fails for the `shell.exec`
reason of [devlore-cli#799](https://github.com/NobleFactor/devlore-cli/issues/799) and
[#801](https://github.com/NobleFactor/devlore-cli/issues/801).

## Implementation Phases

### Phase 1: Commit the plan

- [x] This plan, before the change

### Phase 2: The move

- [x] `git mv` the six files (Requirement 1)
- [x] `README.md` names the new path (Requirement 1)

### Phase 3: The deletion

- [x] `git rm docs/architecture/star-wasm-receivers.md`, its own commit, the reasoning in the
      message (Requirement 2)

### Phase 4: Verify, then merge

- [x] `git ls-files` shows six files at the new path and none at the old
- [x] The gates that run on this host: frontmatter, 57 checked, 5 exempt, 0 errors; PowerShell,
      2 checked, 0 findings
- [x] The six `.star` files are pure renames — `git diff` reports no content lines — and the Starlark
      gate globs `git ls-files '*.star'`, so the move is invisible to buildifier
- [x] Nothing references `star-wasm-receivers.md`: no link here, no `.github/frontmatter-exempt`
      entry, no `.github/codespell-ignore` word keyed to it
- [x] PR script written, analyser-clean, shown, and handed over
- [x] CI green on the pull request: `quality-gate` passed in 31s on PR #230, covering the Starlark,
      spelling and shell checks that buildifier, codespell, shfmt and shellcheck provide — none of
      them installed on this host, and shellcheck has no native ARM64 build

### Phase 5: The live path

Not a handover. [`agent-rules.md`](../../guides/agent-rules.md) rule 5: a layer change is finished
when the machine is converged, not when the pull request merges. This plan reaches `complete` in the
commit that closes these boxes, which is after the merge — #199's rule is "when every other box is
ticked", and these are not tickable before it.

- [x] star rebuilt from devlore-cli `develop` and installed (Requirement 3) — `make install`, build
      `a835739d`, identical to develop's HEAD
- [x] writ's base clone pulled to `9ecae64`; `writ deploy common` — 62 files, 62 links, 1 skipped
- [x] `star gh issues report` resolves from the new path. **Better than this plan predicted:** it does
      not fail for the `shell.exec` reason of devlore-cli#799 and #801 — it runs, exit 0, 278 rows
      across all three repositories. What is wrong with it is the raw-JSON rendering of #190, which
      is a different issue and not a failure
- [x] `Remove-BrokenLinks -Path ~/.local/share/star` cleared the six dangling links, and the warning
      star emitted on the deprecated probe is gone
- [x] `~/.local/share/star` is gone. **Not as this plan assumed:** it was not empty. It held 24 real
      files — the five `com.noblefactor.devlore.*` extensions that the *previous* `star self install`
      wrote to the old path. Today's install wrote fresh copies to the new path and updated its
      manifest, stranding the old ones: after devlore-cli#917 `star self uninstall` removes only what
      its own manifest records, so nothing would ever have collected them. Removed by the owner
      2026-09-23. **The migration gap is real and unfiled** — a path move strands the previous
      install's files, and neither #917 nor #918 covers it

## Out of Scope

**The other six architecture documents, the star-era plans, the two star guides, the WASM example
and `docs/schemas/star-gen-mapping.json`.** The audit found that three of the six describe code that
ships in devlore-cli today and is undocumented there — `doc-comment-styling.md` (the goast styler),
`star-source-analysis-api.md` (five providers absent from devlore-cli's own provider catalog) and
`star-file-tree-walking.md` (`pkg/gitignore`) — so they move rather than die, which makes it a
cross-repo lane with a devlore-cli pull request ahead of a noblefactor-ops one. Filed as #228 and
placed on #217 as lane 16.

**The deprecated probe.** devlore-cli#920 removes it, after every consumer has redeployed.

## Related Documents

- [devlore-cli#918](https://github.com/NobleFactor/devlore-cli/issues/918) — the move; this is its plan's Phase 3
- [devlore-cli docs/architecture/star-extensions.md](https://github.com/NobleFactor/devlore-cli/blob/develop/docs/architecture/star-extensions.md) — scopes from star, ownership from writ
- [#217](https://github.com/NobleFactor/noblefactor-ops/issues/217) — lane 15
