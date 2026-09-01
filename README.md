---
title: "noblefactor-ops"
description: "The organization's documented standards, and the historical record of the star tooling that moved to devlore-cli"
---

# noblefactor-ops

**This repository is private.** It holds documents. It builds nothing.

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

## CI

One job, `quality-gate`: frontmatter validation and spelling. The name is fixed — organization
ruleset 12426847 requires that context on every repository, so renaming it would leave a required
check that never arrives.

## License

MIT. See [LICENSE](LICENSE).
