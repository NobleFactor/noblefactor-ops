---
title: "agent-rules gains rule 12: a document a commit makes stale is corrected in that commit"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/229
status: active
created: 2026-09-23
updated: 2026-09-23
---

# Plan: agent-rules gains rule 12

## Issue 229

Lane 16 of #217, reordered ahead of #228 on 2026-09-23 so that #228 — a two-pull-request,
multi-document change — runs under the written rule rather than an agent's memory of it.

`development-process.md:206` has ruled this since #130. `agent-rules.md` carries eleven rules and not
this one, though its own preamble says it holds "what those instructions assume and an agent still
got wrong". On 2026-09-23 an agent got it wrong.

## Goals

1. Rule 12 is written in the document's established shape, with the breakage and the date.
2. It carries the clause that stops it doing harm: a box that cannot close before the merge is
   declared, not ticked.
3. Nothing else changes. `development-process.md` already states the rule correctly.

## Requirements

### Requirement 1: The rule

Appended after rule 11, before the closing coda:

> ## 12. A document a commit makes stale is corrected in that commit
>
> **The rule.** When a commit changes what a document describes, the correction belongs in that
> commit. A plan's phase box, a status table, a README path, an architecture document's types:
> staged with the change, not in the next commit and not at the end of the branch. Where a box
> genuinely cannot close until after the merge — a deploy, a live-path check — the plan says so in
> its own text and stays `active`; `complete` still means every other box is ticked (#199).
>
> **The breakage.** 2026-09-23, #227. The commit that moved six files in a writ layer, `1c0de72`,
> left the plan's Phase 2 boxes unticked; the tick arrived afterwards, in `9404b72`.
> `development-process.md:206` had ruled it since #130 — "Every commit updates every document it
> touches — plan, architecture, and user-facing material. No deviations" — and the agent read that
> rule only on reaching the next phase, finding its own breach on the way. Its first instinct had
> been to defer the tick to a single plan-status commit at the end of the branch, which is the batch
> `development-process.md:217` names and rejects. Ruled: "please correct your breach of the rules.
> file an issue to correct the mistake."
>
> **Compliance.** Stage the document with the change. Before committing, ask which documents the
> change has made stale and read them rather than recall them — the rule was available the whole
> time and went unread. A plan that cannot close inside its own pull request says why in its own
> text, so a reader can tell a declared box from a forgotten one.

### Requirement 2: The document's own metadata

`updated:` becomes 2026-09-23. Rule 12's first act is to not break rule 12.

## Implementation Phases

### Phase 1: Commit the plan

- [x] This plan, before the change

### Phase 2: The rule

- [ ] Rule 12 after rule 11, before the coda (Requirement 1)
- [ ] Frontmatter `updated` (Requirement 2)
- [ ] Any document stating the rule count is corrected, or confirmed not to exist

### Phase 3: Verify, then merge

- [ ] The frontmatter and PowerShell gates pass on this host
- [ ] #227's owed Phase 5 tick rides in this branch — the deploy is done and verified, and #199's
      rule closes that plan in the next pull request through
- [ ] PR script written, analyser-clean, shown, and handed over
- [ ] Codespell, Starlark and shell run on the pull request, and the PR script's own gate blocks the
      merge until every check reports pass

A box is not written here that cannot be closed here. "CI is green" is an outcome that arrives after
this plan's last commit, so what is stated is the arrangement — which is true when the box is
ticked — and the merge gate is what enforces the outcome. Plan 214 sets this form; rule 12's second
clause is the general case.

## Out of Scope

**`development-process.md`.** The rule is already there and already correct. Restating it in the
engineers' document would be the duplication rule 12 exists to prevent.

**The other eleven rules.** Not reviewed, not renumbered.

## Related Documents

- [#227](https://github.com/NobleFactor/noblefactor-ops/issues/227) — where it was broken
- [#199](https://github.com/NobleFactor/noblefactor-ops/issues/199) — `complete` at the last commit
- [development-process.md § Documents on every commit](../../guides/development-process.md)
