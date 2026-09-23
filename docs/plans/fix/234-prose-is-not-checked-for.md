---
title: "Prose is checked for spelling locale: codespell gains its en-GB_to_en-US dictionary"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/234
status: active
created: 2026-09-23
updated: 2026-09-23
---

# Plan: Prose is checked for spelling locale

## Issue 234

Lane 18 of #217, ahead of #228. Also lane 4 of #232, which owns the theme.

Ruled 2026-09-23: *"we're US … misspell should be enforcing us spelling."* It is, and it is the only
thing that does. `.golangci.yaml:103-104` sets `locale: US` and it caught `behaviour` in a Go comment
on devlore-cli#933 — but it sees Go source only. codespell, which checks prose here, finds typos and
not locale variants, and devlore-cli has no prose gate at all.

## Verified before planning

- The action `codespell-project/actions-codespell@v2` accepts a `builtin` input, "Comma-separated
  list of builtin dictionaries to include", default empty.
- codespell ships `codespell_lib/data/dictionary_en-GB_to_en-US.txt`, so the dictionary name is
  `en-GB_to_en-US`. It holds **535 entries**.
- codespell's own default is `clear,rare`, and the action passes `--builtin` only when set — so the
  value must be `clear,rare,en-GB_to_en-US`, not the dictionary alone, or the defaults are lost.

codespell is not installed on this host and there is no Python, so the sweep was computed by
fetching that dictionary and matching its British side against every tracked file. That is what
codespell will flag.

## Current state

**noblefactor-ops: 28 files, 69 instances.** Far past the markdown-only estimate in the issue (15 and
31), because the dictionary reaches well beyond a hand-written list and codespell checks every file,
not just `.md`:

| Word | Count | | Word | Count |
| --- | --- | --- | --- | --- |
| `catalogue` | 17 | | `labelling` | 3 |
| `analyser` | 9 | | `honours` | 3 |
| `colour` | 8 | | `judgement` | 2 |
| `labelled` | 7 | | `honoured` | 2 |
| `behaviour` | 6 | | `analysed`, `analysing`, `favour`, `marshalling`, `normalisation`, `practised`, `signalling`, `summarised` | 1 each |
| `licence` | 4 | | | |

**devlore-cli: 45 files, 110 instances**, by the same method:

| Word | Count | | Word | Count |
| --- | --- | --- | --- | --- |
| `licence` | 29 | | `acknowledgement` | 3 |
| `behaviour` | 24 | | `labelling`, `labelled`, `judgement`, `honour`, `favour`, `colour` | 2 each |
| `cancelled` | 9 | | `programme` | 1 |
| `neighbours` | 6 | | | |
| `analogue` | 5 | | | |
| `modelled`, `artefacts` | 4 each | | | |

All 29 `licence` are in `docs/plans/licensing-readme-plan.md` and are the author's own prose — "assigns
a licence to each asset class" — not quoted boilerplate, so they correct cleanly. `CONTRIBUTING.md` and
`TRADEMARK.md` are the boilerplate case, in both repositories.

Much of it is recent and mine: `analyser` is largely `docs/plans/chore/218-ci-powershell-gate.md`,
and `favour` is in `docs/plans/chore/227-base-layer-deploys-its-star.md`, written today.

## Requirements

### Requirement 1: The rule is written down

`docs/documentation-standards.md` states that prose is US English. Today the standard exists only as
a linter setting that sees one language's source files.

### Requirement 2: The gate enforces it

`ci.yaml`'s `Spelling` step gains `builtin: clear,rare,en-GB_to_en-US`. The sweep lands in the same
pull request, because enabling the dictionary without it is a red gate.

### Requirement 3: `catalogue` is renamed, vocabulary and all

**Ruled 2026-09-23:** *"rename using the american spelling. all words should be in US English."* No
word is exempted and `.github/codespell-ignore` gains nothing from this issue.

So the document family `documentation-standards.md` defines is the **catalog** family, and
`.github/scripts/Test-Frontmatter.sh` renames with it:

| Was | Becomes |
| --- | --- |
| `CATALOGUE_PATHS` | `CATALOG_PATHS` |
| `VALID_STATUS_CATALOGUE` | `VALID_STATUS_CATALOG` |
| `REQUIRED_CATALOGUE` | `REQUIRED_CATALOG` |
| `family=catalogue` | `family=catalog` |

The `family` value reaches the operator: it is interpolated into the gate's messages — "catalogue
document is missing 'type'" becomes "catalog document is missing 'type'". Nothing outside this
repository reads the constants or the value; devlore-cli's sweep turned up no `catalogue` at all.

`analogue` in devlore-cli goes the same way, to `analog`, under the same ruling.

### Requirement 4: Every instance is corrected, records and legal text included

All 69 here and 110 there, wherever they sit, including closed plan documents: a spelling correction
does not change what a record says, and skipping `docs/plans/**` would leave every future plan
unchecked, which is the gap this issue closes.

`licence` in `CONTRIBUTING.md` and `TRADEMARK.md` is covered by the ruling too — **but those files are
devlore-cli's, and this repository has neither.** Its four are ordinary prose, "took the finding as a
licence" in the permission sense, and `LICENSE` and `NOTICE` carry no British spelling and are
untouched. The legal-text reading belongs to Phase 4, whose pull request names those lines explicitly
rather than burying them in a count.

### Requirement 5: This plan is the one file the gate skips

A document whose subject is the difference between two spellings has to contain both. Its word tables
are a record of what was found, and its rename table is meaningless if either column is rewritten — a
blanket pass over the tree turned `CATALOGUE_PATHS` → `CATALOG_PATHS` into `CATALOG_PATHS` →
`CATALOG_PATHS` on the first run, and the sentence beside it into a tautology.

So this file joins codespell's `skip`, which already carries `./.git,*.svg,*.wasm,*.lock`. That is an
exemption for one document, visible in the workflow, and not a word exempted repository-wide in
`codespell-ignore`: the ruling leaves no word exempt, and this exempts none.

### Requirement 6: devlore-cli is swept, and its gate is lane 2's

devlore-cli's own prose gate arrives with devlore-cli#932, which consolidates document checks into
star. This plan sweeps that repository's files in a second pull request and adds no bash gate there
for #932 to delete.

## Implementation Phases

### Phase 1: Commit the plan

- [x] This plan, before the change

### Phase 2: noblefactor-ops — the rule, the gate, the sweep

- [x] `documentation-standards.md` states the rule (Requirement 1)
- [x] `ci.yaml` gains `builtin: clear,rare,en-GB_to_en-US` (Requirement 2)
- [x] `catalogue` renamed to `catalog`, constants and family value included (Requirement 3)
- [x] Every remaining instance corrected (Requirement 4)
- [x] This plan added to codespell's `skip` (Requirement 5)
- [x] The dictionary re-run over the tree: zero hits outside this plan

### Phase 3: Verify, then merge

- [x] The frontmatter and PowerShell gates pass on this host -- 59 checked, 5 exempt, 0 errors; 2 checked, 0 findings
- [x] `Test-Frontmatter.sh` still passes after its three constants and its `family` value are renamed, and a bad fixture still fails it -- "catalog status ... is not one of", exit 1
- [x] PR script written, analyzer-clean (parse errors 0, findings 0, no BOM), shown, and handed over
- [x] Codespell, Starlark and shell run on the pull request, and the merge gate blocks until every
      check reports pass

### Phase 4: devlore-cli — the sweep

Another repository, so another pull request, and this plan cannot close in the one that carries
Phases 1-3. It stays `active` until these boxes close — #199's rule is `complete` "when every other
box is ticked", and rule 12's second clause is that a plan which cannot close inside its own pull
request says so in its own text.

- [ ] The same dictionary pass over that repository, in its own pull request (Requirement 6)
- [ ] `make vet`, `make lint`, `make test`

## Open Questions

- [x] **`catalogue` — ignore or rename?** Renamed. Ruled 2026-09-23: all words in US English, no
      exemptions.
- [x] **`analogue` in devlore-cli — the same question.** Renamed, under the same ruling.
- [ ] **`honours` in the deployed extension.** Two of the three are in
      `com.noblefactor.ops.GitHub` — `gh-labels-sync.star` and `extension.yaml` — which writ deploys.
      Correcting them is a layer change, so rule 5 applies: it is not done until the deploy has run.
- [x] **`licence` in legal text.** Corrected with the rest, under the same ruling. The pull request
      names the changed lines in `CONTRIBUTING.md` and `TRADEMARK.md` explicitly, so the two files
      where the word carries legal weight are reviewed rather than counted.

## Related Documents

- [#232](https://github.com/NobleFactor/noblefactor-ops/issues/232) — the lint tooling schedule, lane 4
- [NobleFactor/devlore-cli#932](https://github.com/NobleFactor/devlore-cli/issues/932) — where devlore-cli's prose gate comes from
- [NobleFactor/devlore-cli#933](https://github.com/NobleFactor/devlore-cli/issues/933) — where `misspell` caught the one it could see
