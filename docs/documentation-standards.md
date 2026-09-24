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

A document that declares `type:` is a **catalog document** — an ADR, an RFC, a process record. Its
`status` describes where a *decision* stands.

A document that does not declare `type:` is a **working document** — a plan or an architecture note.
Its `status` describes where the *work* has got to, and its shape is prescribed by
[docs/plans/TEMPLATE.md](plans/TEMPLATE.md), the template CLAUDE.md mandates org-wide.

| | Working | Catalog |
| --- | --- | --- |
| Discriminator | no `type` | declares `type` |
| Required | `title` | `title`, `type` |
| `status` | `draft`, `approved`, `active`, `complete`, `abandoned` -- the list in [docs/plans/TEMPLATE.md](plans/TEMPLATE.md) | `Draft`, `Proposed`, `Decided`, `Approved`, `Accepted`, `Placeholder`, `Superseded`, `Withdrawn`, `Research Complete` |
| Example | `docs/plans/*` | ADR trees, `docs/guides/pr-script-template.md` |

`type` must be one of: `ADR`, `RFC`, `PRD`, `README`, `Overview`, `Pitch`, `Index`, `Demos`,
`Strategy`, `Roadmap`, `Reference`, `Guide`, `Demo Script`, `Plan`, `Process`, `Brief`.

## Why `type` is the discriminator

Not directory. `docs/guides/` in this repository holds both families: `pr-script-template.md` and
`release-process.md` are `type: Process` with `status: Approved`, while `go-style-guidelines.md`
carries neither. A path rule would misclassify one or the other.

Using `type` makes each document self-describing, so moving a file does not change what it must
satisfy.

`CATALOG_PATHS` in the script pins chosen paths to the catalog family regardless, so a tree of
ADRs cannot silently downgrade itself by omitting `type`. It is empty here; a repository holding an
ADR tree should list it.

## Statuses that mean the same thing

Two overlaps are accepted rather than normalized, because settling them is an editorial decision:

- Catalog: `Decided`, `Approved`, and `Accepted` all read the same to a reader.
- Working: both `approved` and `Approved` appear in this tree, as does `active`.

## Exemptions

A file that legitimately carries no frontmatter is listed in `.github/frontmatter-exempt`, one path
per line as `git ls-files` prints it. Exemption is for repository furniture — license text, a README
that is a landing page — and for closed historical records whose paths now dangle. It is not a way
to skip filling in a field.

A new entry is a decision a reviewer should see, which is why it is a list and not a pattern.

## Documents are US English

Ruled 2026-09-23 (#234): *"we're US … all words should be in US English."* `-or` and not `-our`,
`-ize` and not `-ise`, `-yzer` and not `-yser`, one `l` before a suffix, and `license` for the noun
as well as the verb.

**No word is exempt, including a word that names something.** The catalog family named at the top of
this document was spelled the other way until that ruling, and it was renamed with everything else —
as were the three constants in `.github/scripts/Test-Frontmatter.sh` that carry the name, and the
value the gate prints in its own messages. That a term is domain vocabulary is not a reason to keep
it, and `.github/codespell-ignore` is not where such a reason goes: that file is for words codespell
gets wrong, not for words we would rather not change.

The gate is codespell's `en-GB_to_en-US` dictionary, added to `clear,rare` in `ci.yaml`. It reads
every tracked file rather than only markdown, so a shell script's comments and a Starlark docstring
are held to the same rule as a plan. One document is skipped by name, #234's plan, because its whole
subject is the difference between the two spellings and it has to quote both.

`golangci-lint` enforces the same rule inside Go source through `misspell` with `locale: US`, and did
so before this was written down. Until #234 it was the only thing that did, which is how the drift
got in.

## What the gate is for

It stops **new** drift. The vocabularies above were enumerated from what the trees already used, so
that the gate is green on the day it lands. A gate that is red on arrival does not get adopted; it
gets bypassed.
