---
title: "The star record is three different things: the rest goes"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/228
status: active
created: 2026-09-24
updated: 2026-09-24
---

# Plan: the rest of the star record goes

## Issue 228

Step 2, and **lane 19 of #217 — the last lane of that schedule.** Step 1 merged as
devlore-cli#942: the three documents that describe code shipping there are now `9.1`, `9.2` and `9.3`
under its numbered §9, with the two salvaged sections in `9-star-extensions.md` and
`configuration.md`.

star was a product of this repository until March 2026. #130 removed the Go and Starlark on
2026-08-27, arguing that *"a diverged second copy is how the next person builds the wrong thing,"*
and left the design documents with a README section saying the reasoning outlived the code. The
2026-09-23 audit found that sentence covering three different things. Two of them are now in
devlore-cli. This removes what is left.

## What goes

**Six architecture documents.** All of them; nothing here survives.

| Document | Why it goes |
| --- | --- |
| `doc-comment-styling.md` | in devlore-cli as `9.1` |
| `star-source-analysis-api.md` | in devlore-cli as `9.2` |
| `star-file-tree-walking.md` | in devlore-cli as `9.3` |
| `star-extensions.md` | superseded there; "The Shape of the Tree" salvaged into `9-star-extensions.md` |
| `star-configuration.md` | superseded by its `configuration.md`; two tables salvaged there |
| `star-goast-linter.md` | a retired participle implementation — `participle` is absent from devlore-cli's `go.mod`, and its own successor supersedes it in frontmatter |

**Nine star-era plans:** `comment-taxonomy.md`, `golangci-canonical.md`, `lint-go-style.md`,
`nil-production-comment-removal.md`, `shared-provider-receivers.md`,
`shared-provider-receivers-phase5-analysis.md`, `star-application-restructure.md`,
`star-cli-syntax-cleanup.md`, `star-consumes-pkg-op.md`.

**Two root drafts:** `PLAN-starlark-extensions.md` — "Starlark Extension Model for **nf-ops**",
commit #4, a tool the README says was never built — and `draft-star-lint-deadcode.md`. #130's open
question at `remove-go-code.md:108`, *"archive under `docs/`, or delete?"*, is answered here.

**Two guides:** `writing-extensions.md` and `configuration.md`, both wholly star-authoring and both
orphaned from the README's standards table.

**One schema and one example:** `docs/schemas/star-gen-mapping.json`, emitted by the `go.mapping()`
builtin and referenced by nothing in either repository; and `docs/guides/examples/wasm-receiver/`,
six files — a working Rust/WASM receiver for the subsystem #230 deleted the design for, reachable
only from `writing-extensions.md`.

## What stays, and why

**`docs/plans/remove-go-code.md`.** Its subject is star-era, but it is the charter for the CI gate
this repository runs today: the `quality-gate` job name required by ruleset 12426847, the frontmatter
and codespell steps, and `.github/frontmatter-exempt` itself.

**`docs/plans/feature/140-star-report-epics.md`.** `status: active`, and `star/config.yaml:5` cites it
by path. It designs `com.noblefactor.ops.GitHub`, which this layer ships.

## The pointers, which must move in the same commit

Six things reference what is being deleted. A gate left pointing at an absent file is the failure this
issue exists to stop.

| Pointer | What happens |
| --- | --- |
| `.github/frontmatter-exempt` | four of its five entries name files being deleted; only `go-style-guidelines.md` stays |
| `.github/codespell-ignore` | `runn` is whitelisted **because** `PLAN-starlark-extensions.md` contains it; the entry goes with the file |
| `docs/documentation-standards.md:31,39-40` | uses these documents as the normative example of the working-document family, and the two star guides as the type-less half of its `type`-discriminator argument |
| `README.md:37-44` | the whole "record of the star work" section, which exists to explain what is going |
| `.github/workflows/ci.yaml:4-9` | a header comment saying this repository holds the star record |
| `docs/plans/feature/140-star-report-epics.md` | six citations of `star-extensions.md`, repointed at devlore-cli's `9-star-extensions.md` |

## #234's plan closes here

It reads `active` behind a closed issue because its Phase 4 landed in devlore-cli. All three boxes are
now true: the sweep merged as devlore-cli#936, its gates passed, and `honours` reached the live path
when the deploy ran on 2026-09-24. This is the next noblefactor-ops pull request through, which is
where #199 says it closes.

## Implementation Phases

### Phase 1: Commit the plan

- [x] This plan, before the change

### Phase 2: The deletions and their pointers, one commit

- [x] Six architecture documents, nine plans, two root drafts, two guides, the schema, the example — 26 files
- [x] The six pointers above, in the same commit — rule 12. Plus `remove-go-code.md`, whose two open questions this answers

### Phase 3: #234 closes

- [x] Its three boxes ticked and its plan set `complete` — the third was a rule 5 breach, recorded as one

### Phase 4: Verify, then merge

- [ ] Nothing tracked references a deleted path
- [ ] The frontmatter and PowerShell gates pass on this host
- [ ] The `en-GB_to_en-US` dictionary over every changed file: zero. codespell is not installed here
      and **it cannot see dot-paths at all**, so `.github/` changes are checked by hand or not at all
- [ ] PR script written, analyzer-clean, shown, and handed over
- [ ] Codespell, Starlark and shell run on the pull request, and the merge gate blocks until every
      check reports pass

### Phase 5: #217 closes

- [ ] #217 set complete — this is its last lane

## Related Documents

- [devlore-cli#942](https://github.com/NobleFactor/devlore-cli/pull/942) — step 1, which received the three
- [#130](https://github.com/NobleFactor/noblefactor-ops/pull/130) — the Go removal, and the reasoning this extends to `docs/`
- [#234](https://github.com/NobleFactor/noblefactor-ops/issues/234) — the plan closing here
