---
title: "star gh issues report: the epic report as a base-layer extension"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/140
status: in-progress
created: 2026-09-05
updated: 2026-09-05
---

# Plan: star gh issues report: the epic report as a base-layer extension

## Summary

`Get-EpicReport` becomes a star extension owned here and deployed by writ, and the bash script retires.
The extension reads the classification scheme from labels across a configured set of repositories,
renders it by epic, by feature or by thread, names why any level is empty, and treats the thread
issue's beat table as the source of order. It lands under a `gh` service node in star's command tree,
whose grammar this plan also writes down. Seven phases; the first reproduces today's table from one
repository and is useful on its own.

## Goals

1. **Reproduce the report** — `gh issues report --by epic --view table` renders what the script
   renders today.
2. **Span repositories** — a thread with members in two repositories reports as one thread.
3. **Implement the scheme as written** — five kinds, threads, the `Ops:` axis, three empty states.
4. **Settle the tree** — the grammar `star <service> <resource> <verb>`, written into
   `star-extensions.md` so the next extension does not have to infer it.
5. **Retire the script** — devlore-cli#797 lands with the last phase.

## The command tree

Ruled 2026-09-05. The grammar is **`star <service> <resource> <verb>`** — the shape
`az keyvault certificate create` has and `star devlore actions generate` already follows. Star's own
commands elide the service token because star is the service. A noun that belongs to no service —
`lint` drives shellcheck, `session` drives tmux — is bare.

```
star
├── config · docs · hook · key · lint · self · setup · version       star's own — <resource> <verb>
│
├── devlore                                                          devlore
│   └── actions · knowledge · model · package · test   <verb>
│
├── gh                                                               ops — this plan
│   ├── issues    report · audit
│   ├── labels    audit · sync
│   └── secrets   rotate                                             #15, the precedent
│
└── session   start [app]                                            ops — #151
```

Owners are visible in the extension prefix — `com.noblefactor.star.*`, `.devlore.*`, `.ops.*` — and
invisible to the person typing. The same rule keeps tool names out of the tree: `gh` appears because
GitHub is a *service*, the way `devlore` is; `shellcheck` does not, because it is a tool `lint` uses.

## Verified facts the design rests on

Read from devlore-cli on 2026-09-05; each is a thing the plan would otherwise have to guess.

| Fact | Where |
| --- | --- |
| `run(command, ctx)`'s return value is the command's result; the shared root emits it through the pipeline, so `-o json`, `-o table`, `-o csv`, `-o yaml` are free | `cmd/star/star/command.go:87-96`; `cmd/star/main.go:435` `cli.Emit(c, result)` |
| Extension search order: `<git root>/star/extensions` → `$XDG_DATA_HOME/star/extensions` (default `~/.local/share`) → `/usr/local/share/star/extensions` → embedded; first name wins | `cmd/star/star/loader.go:232`; `pkg/xdg/xdg.go:148` |
| The project config file is **`star/config.yaml`**; an extension declares `config: path: <name>` and its fields | `devlore-cli/star/config.yaml`; `LintShell/extension.yaml` |
| An extension is `extension.yaml` plus `commands/<name>.star`; command `gh.issues.report` becomes `star gh issues report` | `cmd/star/extensions/com.noblefactor.star.LintShell/` |
| `shell.exec(command)` runs `sh -c` and returns stdout, stderr and the exit code; `json.decode(text)` parses | `pkg/op/provider/shell/provider.go:49`; `pkg/op/provider/json/provider.go:38` |
| `ctx.args.get(name, default)` reads flags and positionals; `note`, `warn`, `error`, `succeed`, `fail` narrate on stderr | `LintShell/commands/lint-shell.star` |

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| `Get-EpicReport` | Working, single-repo | ~200 lines of jq in 350 of bash; chores render since devlore-cli#809 |
| `--by thread` | Inert | selects epics from a hardcoded list of four; prints "No epics matched" here |
| Cross-repository reach | None | `gh issue list` sees one repository |
| Empty-level naming | None | an epic with no features renders blank |
| `Ops:` sectioning | None | |
| Feature rollup | None | flat issue list only |
| Label-set enforcement | None | `issue-standards.md`: "nothing currently checks this" |
| Tree grammar | Inferred, unwritten | `star-extensions.md` names commands, not the shape of the tree |

## The extension

**Name** `com.noblefactor.ops.GitHub` under `Home/common/.local/share/star/extensions/` — `common`,
because star runs on Windows. writ deploys it to `$XDG_DATA_HOME/star/extensions/`, the user slot.
One package holds every `gh` resource.

### `star gh issues report`

| Flag | Values | Default |
| --- | --- | --- |
| `--by` | `epic`, `feature`, `thread` | `epic` |
| `--view` | `tree`, `table` | `table` |
| `--epic <Name>`, `--thread <Name>` | narrows to one | all |
| `--state` | `open`, `closed`, `all` | `open`; `all` when `--by thread` |
| `--repo <owner/name>` | repeatable; narrows the configured set | the configured set |
| `--directory <path>` | a working tree; resolves to its repository and narrows to it. `-C` when the spec grows short flags | — |

**Result** — one row per issue, in tree order. Phase 1 ships the jq's row, unchanged, because parity
is measured against it:

```
{issue, url, title, kind, state, done, placement, severity, priority, wip, faults[], comment, epic, feature}
```

The target shape adds `repo` (Phase 2) and `threads[]` (Phase 3). `issue` stays `issue`; renaming it
`number` buys nothing and breaks the one consumer that exists.

`-o json` is what a script consumes; `-o table` renders the rows, columns alphabetical. The markdown
document — headings per section, the table beneath, byte-identical to the script's — is a **string
result** behind `--markdown`, and a string result renders quoted under the default json, so the
working form is `--markdown -o value`. That tail is the cost of the pipeline having no text
passthrough for a scalar string; the devlore-cli issue filed from Phase 1 is the fix, and until it
lands the PR template carries the long form.

### `star gh issues audit`

The classification audit alone, as a command rather than a flag: every open issue the scheme cannot
place, with the fault. Same repository set, same `--repo` and `-C`.

### `star gh labels audit` · `star gh labels sync`

`audit` reports which configured repository lacks a kind, `Epic:Ops:Process`, or a thread label that
reaches it. `sync` creates what is missing, with the canonical colour and description. One reads,
one acts — the same pair as `issues audit` and `issues report`.

### Config, in `star/config.yaml`

```yaml
gh:
    repositories:
        - NobleFactor/devlore-cli
        - NobleFactor/noblefactor-ops
    exempt: [65]
```

Flat. The plan first had `gh: issues: exempt:`; the extension spec accepts a `nested:` block, and the
runtime does not build it into the config struct (Phase 1 finding below).

A repository that declares nothing reports on the repository it is run in — the degenerate set of one.

## Implementation Phases

### Phase 1: Reproduce the report from one repository, and write the grammar down — complete 2026-09-05

The jq ported to Starlark, against `gh issue list --json` for the current repository only. The shape
of every later phase is fixed here: the row schema, the kind and placement rules, the audit.

- [x] `extension.yaml`, `commands/gh-issues-report.star`, `commands/gh-issues-audit.star`, the `gh`
      config block
- [x] Rows built from `gh issue list`; `--by epic`, `--view table|tree`, `--epic`, `--state`
- [x] Five kinds; a chore is a child of its feature; unparented children are `unfiled`
- [x] `star-extensions.md` §Naming Convention states the tree grammar and the service-token rule
- [x] **Acceptance:** `star gh issues report --epic Ops:Process --view table --state all` run in this
      repository is row-for-row what `Get-EpicReport` produces today
- [x] **Beyond acceptance:** devlore-cli, `--view tree --state all`, with `gh.exempt: [65]` supplied
      through the XDG user config -- PARITY OK (775 lines). That run exercises severity, priority, WIP, exemption
      and the triage queue, none of which this repository's issues do.

**Files**: `Home/common/.local/share/star/extensions/com.noblefactor.ops.GitHub/**` — Create;
`docs/architecture/star-extensions.md` — Modify

### Phase 2: The configured set of repositories — complete 2026-09-05

- [x] Stage 1: one GraphQL request, one alias per configured repository, all labels with open counts;
      filtered client-side on `^(Epic|Thread):` — never with `labels(query:)`, which is a relevance
      search that returned `feature` for `"Epic:"` and nothing for `"Ops:"`
- [x] Stage 2: **per-repository `gh issue list -R`, not `gh search issues`** — see the decision below
- [x] Normalisation unnecessary under that decision; `gh issue list` returns the shape the scheme already reads
- [x] `--repo` comma-separated (no slice flag type in the spec); `--directory` resolves a working tree to its repository
- [x] The banner names the repositories queried; with `gh issue list` there is no eventual consistency to warn of
- [x] **Acceptance:** `Epic:Ops:Process` reports devlore-cli#809 under it — row 20 of 20, from noblefactor-ops with both repositories configured
- [x] Single-repository parity unchanged: 27 / 67 lines here, 786 on devlore-cli, byte for byte

### Phase 3: Threads — complete 2026-09-05

- [x] Threads discovered from `Thread:<Name>` labels in stage 1; `thread_epics` has no successor
- [x] The thread issue found by label, kind `feature`, and a title beginning `Thread:`; its beat table parsed for order — rows whose first cell is a number, first issue reference on the row
- [x] Agreement audit: labelled-but-not-listed and listed-but-not-labelled are both faults, rendered under the thread's table. Both threads are clean today, so the fault paths are exercised by reading, not by a live case
- [x] **Acceptance:** `star gh issues report --by thread --thread Ops:PortableTooling` renders the thread
      issue and nine members across two repositories in beat order 0–8, with each member's owning epic

### Phase 4: The scheme's semantics — complete 2026-09-05

- [x] `--by feature`: one row per feature — done, open, and its state; defaults to `--state all`, since a done-versus-open rollup that cannot see closed issues counts nothing as done
- [x] Empty levels named: *awaiting plan and design*, *awaiting decomposition*, *unfiled* — in the tree, in the epic row's comment, and in the feature rollup's state
- [x] `Ops:` sectioned apart from product in every view: product first, tooling last, a divider between when both are present; rows carry `axis`
- [x] **Acceptance:** `--by feature --epic Ops:Process` renders #142's six features with counts and states — 1/0 children done, 4/0 children done, 2 of 4, 0 of 3, awaiting decomposition, and the thread issue named as one — and no blank section anywhere

### Phase 5: The label set — complete 2026-09-05

- [x] `gh labels audit`: for each configured repository, the kinds, `Epic:Ops:Process`, and every
      thread label present anywhere in the set; reports what is missing and what has drifted in colour
      or description
- [x] `gh labels sync`: create what `audit` reports missing and align what drifted, canonical colour
      and description; never deletes; honours `--dry-run`
- [x] **Acceptance:** a throwaway `Thread:Ops:Scratch` created in one repository was reported missing in
      the other, `sync --dry-run` said what it would do and did nothing, `sync` created it, `audit` came
      back clean, the throwaway was deleted from both (no issues carried it), and `audit` stayed clean

### Phase 7: Schedules — complete 2026-09-05

Added after Phase 5. Epics say who owns work; threads say what scenario it serves; **a schedule says
what we are executing, in what order, and what is next.** The three are peers (#173,
`issue-standards.md` §Three ways to slice the work). #172 is the task; #171, *Schedule: the four
lanes*, is 768's Requirement 2 as an issue the report can read.

- [x] Schedules discovered by kind `chore` and title `Schedule:`; `--schedule <name>` narrows by the
      name after the prefix
- [x] The lane table parsed: rows whose first cell is a lane number; the item is the `Epic:<Name>`
      token if the cell has one, else its first issue reference; next and waits-on rendered as written,
      with `owner/repo#N` shortened for configured repositories
- [x] Progress derived: a lane naming an epic aggregates its features — *n of m done across k
      feature(s)*; a lane naming a feature rolls up its children; anything else reports its own state
- [x] `-o json` rows carry `schedule`, `lane`, `item_ref`, `item_kind`, `done`, `open`, `features`,
      `state`, `next`, `waits_on`
- [x] **Acceptance:** `--by schedule` renders #171's four lanes — #740 at 13 of 22 across 4 features,
      ResourceModel at 32 of 41 across 8, #762 at 0 of 4, #441 at 0 of 7 across 3 — with the declared
      next and waits-on beside each

**Decisions.** Order, next and waits-on are declared; only progress is derived — the rule threads
taught, and the reason inferring "next" was rejected. A lane's item prefers `Epic:<Name>` over an
issue reference when a cell carries both, since that is what 768 meant by "Resource management —
`Epic:ResourceModel`, #625 at …". No new verb and no new label family: `--by schedule` is a value,
and schedules are found by title.

**#169, done here.** `gh issues audit --unthreaded [--epic]` lists open tasks, bugs and chores
carrying no `Thread:` label, grouped by epic and feature — a requested validation, not a fault, and
not part of the default audit. Inference from shared features or body mentions was rejected as
evidence-free. Across the set today: 152 open issues in 22 epics, and zero under `Ops:Process`,
where every open child is in `Thread:Ops:PortableTooling`.

**Finding.** buildifier sorts `load()` symbols, so a textual edit that targets the load line as
written before formatting will not match after it; patch that line structurally.

### Phase 6: Retire the script — this repository's half complete 2026-09-05; devlore-cli's half waits on deployment

- [ ] devlore-cli#797: `scripts/Get-EpicReport` deleted; devlore-cli's `star/config.yaml` declares
      its `gh:` block. **Gated on the extension being reachable without `XDG_DATA_HOME`** — the user
      slot holds only devlore's five extensions, and writ's `base` layer is still an empty directory.
      Three routes, the choice recorded when made: `writ repo add base` and deploy; a symlink into the
      user slot as writ would place it; or wait for devlore-cli#475
- [x] `development-process.md` names `star gh issues report` as the report, by epic, feature, thread or schedule
- [x] The PR script template's closing report line becomes `star gh issues report ... --markdown -o value --silent`; `issue-standards.md` no longer names the script

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `Home/common/.local/share/star/extensions/com.noblefactor.ops.GitHub/extension.yaml` | Create | four commands, their flags, the `gh` config schema |
| `.../commands/gh-issues-report.star` | Create | Phases 1–4 |
| `.../commands/gh-issues-audit.star` | Create | Phase 1 |
| `.../commands/gh-labels-audit.star`, `gh-labels-sync.star` | Create | Phase 5 |
| `docs/architecture/star-extensions.md` | Modify | Phase 1: the tree grammar |
| `docs/plans/feature/140-star-report-epics.md` | Create | this plan |
| `docs/guides/development-process.md`, `docs/guides/pr-script-template.md` | Modify | Phase 6 |

## Related Documents

- Issue #140 — this feature; its body is the design this plan implements
- Issue #151 — `star session start`, the other `ops` noun in the tree
- Issue #152 — `Thread:Ops:PortableTooling`, beat 3, and the Phase 3 acceptance case
- Issue #15 — `star gh rotate-secrets`, closed; the precedent for a `gh` node, now `gh secrets rotate`
- `docs/issue-standards.md` — the scheme; §Threads, §The Ops axis, §What an empty level means
- `docs/architecture/star-extensions.md` — the extension specification
- `NobleFactor/devlore-cli#797` — the companion that deletes the script
- `NobleFactor/devlore-cli#809` — the last change to the script, and Phase 1's parity target

## Phase 1 findings

Three things the runtime does that the plan could not know until code ran against it. None blocks;
each is recorded so a later phase or a devlore-cli issue picks it up rather than rediscovering it.

- **A string result needs `-o value`** (devlore-cli#826). `print()` narrates to stderr and the pipeline's default is
  json, so the markdown document — which must be a string — is quoted under the default. `--markdown
  -o value` is the working incantation and is what the parity diff uses. Phase 6's report line in the
  PR template will carry it. A `text` passthrough rendering for scalar strings is the fix and belongs
  to devlore-cli's CLI epic.
- **Dict columns render alphabetically.** `-o table` on a list of dicts orders columns by the sorted
  key union; the only way to control order is a Go type implementing `HasHeaders`, which Starlark
  cannot produce. The JSON rows are the contract; the human table is `--markdown`.
- **An empty list becomes `null`** (devlore-cli#825). `goValue` converts an empty Starlark list to a nil slice, so
  `"faults": []` marshals as `"faults": null`. A consumer testing `length` sees the difference. A
  devlore-cli fix in `goValue`.
- **No short flags in the extension spec** (devlore-cli#827). `Flag` has `name`, `type`, `help`, `default`, `required`;
  no alias. `-C` ships as `--directory` until the spec grows one.
- **`nested:` in an extension's config schema is parsed and not built** (devlore-cli#823). `ConfigSchema.Nested` exists
  and `ToConfigSpec` copies it, but the struct `config.get` exposes carries only the flat `fields`.
  Declared `gh: issues: exempt:`; observed `dir(cfg.gh) == ["repositories"]`. Flattened to
  `gh.exempt`. A devlore-cli fix, or the `nested:` key should go from the spec.
- **`[]int` is a valid field type**, alongside `[]string`.
- **`shell.exec` raises on a non-zero exit** (devlore-cli#824) instead of returning the result, so a Starlark caller
  never sees `exit_code` or `stderr` for a failed command -- `gh`'s own message is lost and the user
  gets `Error in shell.exec: exit status 4`. The `exit_code` checks in `scheme.star` are therefore
  unreachable and kept only as intent. Surfacing the captured stderr belongs to devlore-cli#800's
  neighbourhood in the shell-provider epic.
- **`load()` resolves against the extension root**, not the loading file's directory:
  `load("commands/scheme.star", ...)`.

## Phase 5 decisions and findings

- **Thread labels are set-wide.** The plan said "every thread label whose members reach it", which is
  circular: members reach a repository only if the label exists there to be applied. A thread crosses
  repositories by design, so a `Thread:` label present in any configured repository is expected in all
  of them. `Epic:<Name>` labels stay per-repository and are not synced.
- **Canonical is first-configured.** Colour and description come from the first repository in
  `gh.repositories` order that carries the label. The first real run found `chore` divergent here
  and aligned it to devlore-cli's `#BFD4F2 Maintenance work on the repo: build, CI, tooling,
  process`. That description predates `issue-standards.md`'s definition of a chore; settling the
  canonical text is a follow-on, and once one repository carries it `sync` spreads it.
- **`sync` never deletes.** Deleting a label strips it from every issue carrying it — data loss, not
  this command's to do. It creates and aligns; a stray label is reported by nothing yet.
- **`--dry-run` is honoured** through `ctx.dry_run`, which star's root flag already sets.
- **First 100 labels per repository.** One GraphQL page; a repository over that is warned about.
  Neither is near it.

## Phase 4 decisions and findings

- **Parity with the script ends here, by design.** An empty level now has a name where the script
  was silent: `_Awaiting decomposition: no tasks filed._`, `_Awaiting plan and design: no features
  filed._`, and `; awaiting plan and design` on an empty epic's table row. The PR script's parity
  gate narrows to the one invocation that must not change, `--epic Ops:Process --view table`.
- **`--by feature` defaults to `--state all`.** Found by the acceptance itself: under the by-epic
  default of `open`, #156's four closed chores were invisible and it read `0 / 0 / awaiting
  decomposition`.
- **A thread issue is not a feature awaiting decomposition.** Its members report under `--by
  thread`; the rollup says so rather than counting zero children against it.
- **Feature states:** `closed`; `awaiting decomposition`; `children done` (all closed, feature open
  — the signal to close it); `n of m done`.
- **A Phase 2 omission fixed here:** the table document's section heading printed `#N` rather than
  the repository-qualified ref, visible only in the two-repository view.

## Phase 3 decisions and findings

- **This repository gains `star/config.yaml`** naming both repositories, so the standing report and
  the thread report span the set by default. Unplanned; the plan named devlore-cli's config and not
  this repository's own. Single-repository parity is now checked with `--repo` forcing one.
- **`--state` defaults by view.** `open` by epic, `all` by thread — the script's rule — implemented
  by an empty default resolved after `--by` is known, so an explicit `--state open` by thread works.
- **The thread table has a Beat column and an Epic column.** The thread report's value is the order
  and the ownership; the epic table's five columns do not carry either.
- **Two conventions became rules** in `issue-standards.md` §Threads, because the parser depends on
  them: the thread issue's title begins `Thread:`; a beat row's first cell is its number and the first
  issue reference on the row is its member.

## Phase 2 decisions and findings

- **Stage 2 is per-repository `gh issue list`, not `gh search issues`.** The plan chose search
  because it spans repositories. But the repository set is *configured* — explicit and small — so
  search's one advantage, not knowing the set, does not apply, while all four of its traps do: index
  lag, the 1000-result cap, lowercased `state`, pre-flattened `labels`. N calls to `gh issue list -R`
  are authoritative, immediate, uncapped per repository, and return the shape the scheme already
  reads. GraphQL stays for stage 1: the label space with open counts, one request, used to validate
  `--epic` against every configured repository and to name what was queried.
- **The `**Feature:**` marker is the last one in the body.** #141 cites
  `**Feature:** NobleFactor/noblefactor-ops#140` in prose — an example of the cross-repository form,
  in the issue about the standard — and carries its real marker on the closing line. The script's
  regex required `#` immediately after the space and so never saw prose citations; a regex that
  admits `owner/repo#N` does. Last match is the rule, written into `issue-standards.md` §Placement.
- **Feature identity is `owner/repo#N`.** A bare `#N` is the child's own repository. devlore-cli#797
  names `NobleFactor/noblefactor-ops#140` and now files under it when both repositories are in the
  set.
- **Rendering across repositories.** With one repository the ref is `#N`, byte-identical to the
  script; with several it is `<repo>#N` — `devlore-cli#809` — since `#809` beside `#147` would be
  ambiguous. Rows carry `repo` and `ref`; `feature_repo` joins `feature`.
- **`--repo` is comma-separated.** The extension flag spec has no slice type (devlore-cli#827
  covers short aliases; the same gap). Comma-separated is the pragmatic form and reads as one list.
- **Resolution order:** `--repo`, else `--directory`'s origin, else `gh.repositories`, else the
  repository the command runs in.

## Decisions recorded

- **Epics, threads and schedules are peers — three ways to slice the same work.** Ruled 2026-09-05
  (#173). A work item carries exactly one epic; the organizing issue of a thread (`feature`, title
  `Thread:`) or a schedule (`chore`, title `Schedule:`) is a view of the work, not work, and carries
  none. The audit exempts both; the rollup stops listing thread issues under epics, so the
  "a thread; its members report under --by thread" state is now a guard rather than a row.

- **Plan approved 2026-09-05.** Phase 1 begins on the next branch.

- **`gh`, not `github`.** `az` uses its own CLI name as the service token; `gh` is GitHub's, and #15
  set the precedent. Ruled 2026-09-05.
- **The audit is a verb.** `gh issues audit`, not `gh issues report --audit`. A flag that changes what
  a command *is* — a report into an audit — is a second command.
- **`labels` is `audit`/`sync`, not `audit --fix`.** Consistent with `issues audit`/`issues report`:
  one command reads, another acts.

## Open Questions

- [ ] **Beat-table parsing.** A markdown table in an issue body, first column the beat number, issue
      references in a later column. Proposed: parse `#N` and `owner/repo#N` from each row in document
      order; ignore everything else. Fragile by construction and stated as such.
- [ ] **Does `gh secrets rotate` live here?** #15 is closed and the extension can hold it, but nothing
      has asked for it since. Proposed: not until something does; the tree has a place for it.
