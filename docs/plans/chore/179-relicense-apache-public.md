---
title: "Relicense to Apache-2.0 and make the repository public"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/179
status: in-progress
created: 2026-09-06
updated: 2026-09-06
---

# Plan: Relicense to Apache-2.0 and make the repository public

## Summary

This repository is the base layer every NobleFactor machine deploys from, and as of 2026-09-06 it
is an advertisement for DevLore. It goes public. NobleFactor/devlore-cli#478 decided the licensing
on 2026-08-17 and fixed the order — relicense to Apache-2.0 first, publish second, both before the
shared scripts arrive — because a repository is simplest to relicense while it has one copyright
holder and no outside contributors. This plan carries that order out: one PR relicenses and
rewrites the public-facing text; the visibility change is a single command run by hand after it
merges; three settings follow the flip. The readiness questions were measured before the plan was
written, and the answers are in the table below.

## Goals

1. **Apache-2.0, in every place a license is stated.** `LICENSE`, a `NOTICE`, every SPDX header,
   the README's license section, and the two guideline passages that still prescribe `SSPL-1.0`.
2. **A README that reads correctly to a stranger.** It says what the repository is, what it holds,
   and how a machine deploys it — and nothing in it says private.
3. **Public, with the settings a public repository should carry**, and verified by the one property
   #147 and #146 depend on: the raw file URL answers without a token.

## Readiness, measured 2026-09-06

| Question | Finding |
| --- | --- |
| Secrets anywhere in history | none — 197 commits scanned for token shapes, credential assignments and sensitive filenames |
| Copyright not Noble Factor's | none — every header and copyright line is Noble Factor's; nothing vendored |
| SPDX headers to sweep | 14 `MIT`; `.github/workflows/ci.yaml` already reads `Apache-2.0` |
| Stale license prescriptions | `docs/guides/go-style-guidelines.md:7` and `docs/architecture/star-goast-linter.md:501` prescribe `SSPL-1.0`; devlore-cli carries 965 `Apache-2.0` headers and none of `SSPL` |
| Text that says private | `README.md:8` — "**This repository is private.** It holds documents. It builds nothing." — and the description, "Internal operations tooling for DevLore - signing, key management, registry maintenance" |
| Author emails in history | `David@thenobles.us` on 153 commits, `david@example.com` on 12; visible once public; history is never rewritten |
| Wiki | enabled, no content (the wiki repository does not exist) |
| Security features | all disabled — secret scanning, push protection, Dependabot alerts; the org is on the Team plan, so they become available free the moment the repository is public |

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| `LICENSE` | MIT | `Copyright (c) 2025 Noble Factor` |
| `NOTICE` | absent | devlore-cli's names Noble Factor LLC |
| SPDX headers | 14 `MIT`, 1 `Apache-2.0` | the 14: `Test-Frontmatter.sh`, `star/config.yaml`, the six files of `com.noblefactor.ops.GitHub`, the three `wasm-receiver` example files, the three `scripts/setup-*.sh` |
| `README.md` | says private; says MIT | the two tables and the star record are right and stay |
| Visibility | private | devlore-cli is public; personal is private |

## Implementation Phases

### Phase 1: Relicense

- [x] `LICENSE` becomes the Apache-2.0 text devlore-cli carries, byte for byte
      (sha256 `cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30`) — one license
      text across the organization, not two transcriptions
- [x] `NOTICE`, in devlore-cli's form: `noblefactor-ops`, `Copyright 2025-2026 Noble Factor LLC`, and
      the "includes software developed at Noble Factor LLC" line
- [x] The 14 `MIT` headers become `Apache-2.0`; the file list above is the whole sweep
- [x] `go-style-guidelines.md:7` and `star-goast-linter.md:501` say `Apache-2.0`
- [x] README's license section says Apache-2.0
- [x] **Acceptance:** `git grep 'SPDX-License-Identifier: MIT'` is empty, and no header or
      guideline names `SSPL-1.0` — the one mention left, in `docs/plans/star-consumes-pkg-op.md`,
      records the licenses as they were when that plan was written, and a record is not swept

### Phase 2: The public face

- [x] README line 8 becomes the statement of what this is: the NobleFactor base layer — the
      organization's standards, its process tooling, and the star extensions — deployed to every
      NobleFactor machine by DevLore's `writ`. Proposed text:

      > This is the NobleFactor **base layer**: the organization's engineering standards, the
      > tooling that runs its process, and the `star` extensions that ship with them. Every
      > NobleFactor machine deploys it with DevLore's `writ`; every NobleFactor repository's
      > `CLAUDE.md` points into it.

- [x] A short "Deploying it" section: `writ repo add base <url>` and `writ deploy common`, in the
      words the writ guide uses — the part that advertises
- [x] "What is here" gains a row for `Home/`: the layer content, today the
      `com.noblefactor.ops.GitHub` extension, soon `Declare-BashScript` and the `git-*` commands (#147, #146)
- [ ] The description becomes (the PR script runs this after the merge): *The NobleFactor base layer — engineering standards and process
      tooling, deployed by DevLore's writ* (`gh repo edit --description`, in the PR script's
      post-merge steps)
- [x] **Acceptance:** `git grep -i private README.md` is empty; the README is read once, top to
      bottom, as a stranger would

### Phase 3: The flip, by hand, and what follows it

- [ ] After the PR merges and the epic report is read, **you** run:

      ```
      gh repo edit NobleFactor/noblefactor-ops --visibility public --accept-visibility-change-consequences
      ```

      Never from a script. GitHub's consequences: stars and watchers kept, private forks detached,
      content indexed from that moment.
- [ ] Then, three settings, each one `gh api` call listed in the PR script as comments to run:
      secret-scanning push protection on; Dependabot alerts on; wiki off
- [ ] **Acceptance:** `gh repo view --json visibility,licenseInfo` says `PUBLIC` and `apache-2.0`;
      `curl -fsSL https://raw.githubusercontent.com/NobleFactor/noblefactor-ops/develop/LICENSE`
      answers without a token; the repository page shows the Apache-2.0 badge
- [ ] devlore-cli#478 closes, pointing here; #147's plan drops the token question

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `LICENSE` | Modify | Apache-2.0 text, identical to devlore-cli's |
| `NOTICE` | Create | Attribution in devlore-cli's form |
| `.github/scripts/Test-Frontmatter.sh` | Modify | header |
| `star/config.yaml` | Modify | header |
| `Home/common/.local/share/star/extensions/com.noblefactor.ops.GitHub/extension.yaml` and `commands/*.star` (5) | Modify | headers |
| `docs/guides/examples/wasm-receiver/{build.mk,extension.yaml,src/lib.rs}` | Modify | headers |
| `scripts/setup-{azure-swa,github-repo,ground-zero}.sh` | Modify | headers |
| `docs/guides/go-style-guidelines.md` | Modify | the prescribed header is Apache-2.0 |
| `docs/architecture/star-goast-linter.md` | Modify | the parsed example is Apache-2.0 |
| `README.md` | Modify | opening, `Home/` row, deploying section, license section |

## Decisions

- **One license text for the organization.** `LICENSE` is copied from devlore-cli and checked by
  digest, so the two repositories can never disagree by a transcription.
- **History is not rewritten.** The author emails in history become visible; rewriting 197 commits
  to hide them would break every clone and every worktree, and the process forbids it. The
  `thenobles.us` address is yours to attach to the GitHub account or not.
- **The flip is a human's command.** It is outward-facing and irreversible in effect; the PR script
  prints it and stops.
- **The "What is not here" section stays.** It is the record of a five-month mistake and its fix
  (devlore-cli#695); a public reader loses nothing by seeing it and the repository keeps its habit
  of saying what happened.
- **`NOTICE` names `noblefactor-ops`.** Ruled 2026-09-06: this is not DevLore, and the NOTICE
  travels with the repository.
- **The planning record goes public as it stands.** Ruled 2026-09-06: "we might take those
  offline later, but it's good for now." Nothing in `docs/plans/` was found that should not be read.
- **The guideline's `SSPL-1.0` is fixed here, not filed.** `go-style-guidelines.md` is what every
  repository's `CLAUDE.md` points at for Go headers; a public guideline prescribing a license the
  organization does not use is a defect in the thing being published.

## Related Documents

- Issue #179 — this chore; feature #158, epic #142
- NobleFactor/devlore-cli#478 — the decision and the order, 2026-08-17; closes with this
- #147, #146 — the moves that wanted a public base for their lint story
- `Thread:WritOrigin` — a base layer anyone can obtain is a beat of the origin
- `docs/github-branch-protection.md` — unchanged by visibility; the rulesets are organization-level

## Open Questions

None open. Both questions the draft carried were ruled on 2026-09-06 and are recorded under
Decisions.
