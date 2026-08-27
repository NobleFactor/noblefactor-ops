---
title: "Documentation Standards"
type: Process
status: Approved
---

# Documentation Standards

Every tracked markdown file in a NobleFactor repository carries frontmatter, and the values it may
carry come from one of two families. The rule is uniform across repositories; only the vocabulary
differs, and which vocabulary applies is a property of the document, not of the repository.

`.github/scripts/Test-Frontmatter.sh` in this repository is the reference implementation. Repositories
adopting the gate should use that script rather than a variant, so that a document valid in one
repository is valid in all of them.

## The two families

A document that declares `type:` is a **catalogue document** — an ADR, an RFC, a process record. Its
`status` describes where a *decision* stands.

A document that does not declare `type:` is a **working document** — a plan or an architecture note.
Its `status` describes where the *work* has got to, and its shape is prescribed by
[docs/plans/TEMPLATE.md](plans/TEMPLATE.md), the template CLAUDE.md mandates org-wide.

| | Working | Catalogue |
| --- | --- | --- |
| Discriminator | no `type` | declares `type` |
| Required | `title` | `title`, `type` |
| `status` | `draft`, `in-progress`, `complete`, `abandoned` | `Draft`, `Proposed`, `Decided`, `Approved`, `Accepted`, `Placeholder`, `Superseded`, `Withdrawn`, `Research Complete` |
| Example | `docs/plans/*`, `docs/architecture/*` | ADR trees, `docs/guides/pr-script-template.md` |

`type` must be one of: `ADR`, `RFC`, `PRD`, `README`, `Overview`, `Pitch`, `Index`, `Demos`,
`Strategy`, `Roadmap`, `Reference`, `Guide`, `Demo Script`, `Plan`, `Process`, `Brief`.

## Why `type` is the discriminator

Not directory. `docs/guides/` in this repository holds both families: `pr-script-template.md` and
`release-process.md` are `type: Process` with `status: Approved`, while `configuration.md` and
`writing-extensions.md` carry neither. A path rule would misclassify one or the other.

Using `type` makes each document self-describing, so moving a file does not change what it must
satisfy.

`CATALOGUE_PATHS` in the script pins chosen paths to the catalogue family regardless, so a tree of
ADRs cannot silently downgrade itself by omitting `type`. It is empty here; a repository holding an
ADR tree should list it.

## Statuses that mean the same thing

Two overlaps are accepted rather than normalized, because settling them is an editorial decision:

- Catalogue: `Decided`, `Approved`, and `Accepted` all read the same to a reader.
- Working: both `approved` and `Approved` appear in this tree, as does `active`.

## Exemptions

A file that legitimately carries no frontmatter is listed in `.github/frontmatter-exempt`, one path
per line as `git ls-files` prints it. Exemption is for repository furniture — licence text, a README
that is a landing page — and for closed historical records whose paths now dangle. It is not a way
to skip filling in a field.

A new entry is a decision a reviewer should see, which is why it is a list and not a pattern.

## What the gate is for

It stops **new** drift. The vocabularies above were enumerated from what the trees already used, so
that the gate is green on the day it lands. A gate that is red on arrival does not get adopted; it
gets bypassed.
