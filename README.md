---
title: "noblefactor-ops"
description: "The NobleFactor base layer: engineering standards, process tooling and star extensions, deployed by DevLore's writ"
---

# noblefactor-ops

This is the NobleFactor **base layer**: the organization's engineering standards, the tooling that
runs its process, and the `star` extensions that ship with them. Every NobleFactor machine deploys
it with DevLore's `writ`; every NobleFactor repository's `CLAUDE.md` points into it.

## What is here

### The standards CLAUDE.md points at

| Document | Defines |
| --- | --- |
| [docs/documentation-standards.md](docs/documentation-standards.md) | Frontmatter: the two document families and their vocabularies |
| [docs/guides/development-process.md](docs/guides/development-process.md) | How work is organized: one worktree, issue triage, plans, documents on every commit |
| [docs/plans/TEMPLATE.md](docs/plans/TEMPLATE.md) | The plan document every task begins with |
| [docs/guides/pr-script-template.md](docs/guides/pr-script-template.md) | The PR script format |
| [docs/guides/go-style-guidelines.md](docs/guides/go-style-guidelines.md) | Go file layout, naming, comments, tests |
| [docs/github-branch-protection.md](docs/github-branch-protection.md) | Branch protection and required checks |

`.github/scripts/Test-Frontmatter.sh` is the reference implementation of the frontmatter standard.
Repositories adopting the gate should use that script rather than a variant, so a document valid in
one repository is valid in all of them.

### The layer content

`Home/` is what `writ` deploys, laid out as the home directory it lands in. Today it holds the
`com.noblefactor.ops.GitHub` star extension — `star gh issues report|audit` and
`star gh labels audit|sync` — under `Home/common/.local/share/star/extensions/`. `Declare-BashScript`
and the `git-*` process commands arrive with #147 and #146.

### The record of the star work

`docs/architecture/` and `docs/plans/` describe the design and construction of `star` and its
extensions. That code moved to **devlore-cli** in March 2026 and was removed from this repository on
2026-08-27; these documents are the record of how it came to be, so the paths they cite no longer
resolve here. They are kept deliberately — the reasoning outlived the code.

Anything about `star` as it exists today belongs in devlore-cli.

### Infrastructure setup

`scripts/setup-azure-swa.sh`, `scripts/setup-github-repo.sh`, and `scripts/setup-ground-zero.sh`.

## What is not here, despite what this file used to say

Earlier revisions of this README described an `nf-ops` binary with key-ceremony, signing, and
registry-audit commands, and a `cmd/nf-ops` / `internal/{ceremony,signing,index,audit}` layout. None
of that was ever built. The only command this repository ever held was `cmd/star`, a copy left
behind by the March move — which devlore-registry's CI went on building for five months, producing
the wrong binary and the failure tracked as devlore-cli#695.

Release signing and key management remain unimplemented. When they are built, they need an ADR
before a README.

## Deploying it

`writ` is DevLore's environment deployer, part of [devlore-cli](https://github.com/NobleFactor/devlore-cli).
Register this repository as the base layer, then deploy. `common` and the projects named for the
configured layers — here, `noblefactor-ops` — are implicit; nothing is named unless it is extra:

```bash
writ repo set base https://github.com/NobleFactor/noblefactor-ops.git
writ deploy
```

Layers above it — a team repository, a personal one — are registered the same way and deployed
together. `writ` merges them into one home directory, and where two layers provide the same path
the higher layer wins. Until devlore-cli#791 and devlore-cli#850 ship, the binary still spells
these two lines `writ repo add` and `writ deploy common`.

## CI

One job, `quality-gate`: frontmatter validation and spelling. The name is fixed — organization
ruleset 12426847 requires that context on every repository, so renaming it would leave a required
check that never arrives.

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
