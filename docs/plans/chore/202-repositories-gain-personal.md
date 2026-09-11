---
title: "gh.repositories names David-Noble-at-work/personal, so the report and the audit see all three repositories"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/202
status: complete
created: 2026-09-10
updated: 2026-09-11
---

# Plan: gh.repositories gains personal

Lane 1 of `Schedule: process tooling` (#206).

## Problem statement

The work spans three repositories. `star/config.yaml` → `gh.repositories` names two. Every `star gh issues`
invocation that relies on the configuration — the report, the audit, the end-of-PR epic table, every
dedup lookup before filing — is blind to `David-Noble-at-work/personal`. The cost was measured on
2026-09-09: two issues filed in devlore-cli duplicating existing ones, past a lookup that could not see half
the record. A schedule lane pointing at a personal issue renders *"is not in the configured repositories."*

## Goals

1. `gh.repositories` names all three, so the configured commands see personal without a `--repo` flag.
2. What that does and does not deliver is stated from measurement, not assumed — personal follows a
   different labelling scheme, and a config line does not change it.
3. Any document that says "two repositories" says three.

## Before → after, measured 2026-09-10 with `--repo` naming all three (the config line's exact effect)

| Command | Before (two repositories) | After (three) |
| --- | --- | --- |
| `report --by epic` | Header names two; personal absent | Header names three. **No personal epic appears** — personal carries the `Epic:Process` label but no issue of kind `epic` to anchor it, and 0 of its 6 open issues carry a `**Feature:**` marker. `--epic Process` renders *"No epics matched."* |
| `audit` | 2 faults (ops#60, devlore#65) | 5 faults: adds personal#147, #151 (no kind, no epic) and #173 (no epic); 2 personal bugs awaiting triage (#152, #157). **The audit now sees personal's hygiene, which is the point** |
| `report --by schedule` | A lane naming a personal issue would not resolve | #206's nine lanes resolve; a personal lane would too |
| dedup before filing | personal invisible | personal's titles in the set |

## Not this lane's to deliver — and why it is said here

Rendering personal's issues **under an epic** needs personal to carry the scheme: an epic issue behind
`Epic:Process` (or its issues re-labelled under the organization's epics) and `**Feature:**` markers on its
tier-three issues. That is a labelling decision in another repository, made on GitHub, not in this
configuration. This lane makes personal *visible to the audit* — which is what surfaces exactly that gap —
and *addressable by schedules*. Whether personal adopts the scheme is put to the owner, not decided here.

Observed and left alone, per the schedule's commitment rule: `--epic <Name>` for a label that exists with
no epic issue behind it says *"No epics matched"* rather than naming the missing epic issue. Real, small,
and not a lane of #206.

## Implementation Phases

### Phase 1: the plan
- [x] This document, the branch's first commit

### Phase 2: the configuration, and the documents that counted to two — complete
- [x] `star/config.yaml`: `David-Noble-at-work/personal` joins `gh.repositories`, with a comment on the
      different owner and scheme
- [x] Swept `README.md`, `docs/**`, the extension: the only "two repositories" text is in
      `docs/plans/feature/140-star-report-epics.md` — a completed plan, a record, not edited (the #147
      ruling); the extension's help text was already generic. Nothing else to change

### Phase 3: acceptance, live — the table above, without `--repo` — complete
- [x] `star gh issues audit`: 5 faults — personal#147, #151 (no kind, no epic), #173 (no epic) join ops#60
      and devlore#65; #152 and #157 awaiting triage
- [x] `star gh issues report --by schedule --schedule "process tooling"`: nine lanes, header names three
- [x] `star gh issues report --by epic`: header names three; 22 epics, the same 22 — no personal epic
      issue exists, as stated

## Files to Create/Modify

| File | Action |
| --- | --- |
| `docs/plans/chore/202-repositories-gain-personal.md` | Create |
| `star/config.yaml` | Modify — one entry, one comment |
| documents naming the repository count | Modify — per the sweep |

## Decisions

- **Measured with `--repo`, then made the default.** `resolve_repos` treats `--repo` and the config
  identically, so the flag run is the config line's exact effect, observed before it is committed.
- **Personal's classification is not touched here.** Five label edits on another repository's issues are
  GitHub-side writes and a scheme decision for its owner; this lane's PR is one file and the docs that
  counted to two. Filing or fixing anything discovered belongs to a later schedule.

## Related Documents

- #202; #206 (the schedule); #140 (the extension); personal#147, #151, #152, #157, #173 (what the audit now
  sees); #176 (the hierarchy going native, which changes what "classified" means)

## Open Questions

None for this lane. For the owner, outside it: does personal adopt the scheme — an epic issue, `**Feature:**`
markers — or stay a repository the audit reports on and the schedules point at, but the epic tree omits?
