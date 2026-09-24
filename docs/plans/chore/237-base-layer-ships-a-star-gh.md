---
title: "The base layer ships a star gh report skill, and #217's lane table matches the format the report reads"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/237
status: active
created: 2026-09-24
updated: 2026-09-24
---

# Plan: the base layer ships the report skill

## Issue 237

Rule 3 has said since 2026-09-07 that a question about issues is answered by the report. On
2026-09-24 an agent broke it twice in consecutive turns, with the rule written down and the command
working, hand-assembling a schedule from `gh issue view` both times.

A rule in a document has to be recalled. A skill is listed to the agent every session, with its
description, before it does anything.

## Requirements

### Requirement 1: The skill ships from `common`

`Home/common/.claude/skills/star-gh-report/SKILL.md`. writ deploys it to `~/.claude/skills/` on every
machine; `~/.claude/CLAUDE.md` already arrives that way from personal's `Home/common/.claude/`.

Here rather than personal because it is not preference — it applies to every NobleFactor project and
belongs beside `com.noblefactor.ops.GitHub`, which this layer already ships. `common` rather than
`common.Windows`: nothing in it is platform-specific.

**On `common`'s rule.** `common` deploys unconditionally and may depend on nothing outside itself.
The skill names `star gh`, which comes from devlore-cli — but it is a document, not a script that
sources anything, and this layer already ships an extension with the same dependency. The exemption
is the same one the `git-*` commands have: nothing points *out*.

### Requirement 2: What the skill records

Two things that cost time on 2026-09-24, not a restatement of rule 3:

- **`--markdown -o value --silent`, always.** Without it the command returns raw JSON rows, and
  reformatting those by hand is the behaviour rule 3 forbids — so the rule gets obeyed in letter and
  broken in substance.
- **Verify `star` first (rule 1), without installing over someone else's build.** If the installed
  build is `-dirty` or not on `develop` it is another session's work in progress: use it read-only
  and say so. An agent replaced the owner's build that morning to run a read-only report.

It also carries the lane-table caution from Requirement 3, as a general check rather than naming a
schedule that is about to be fixed.

**Its frontmatter carries `title` as well as `name` and `description`.** The gate reads every tracked
`.md` (`git ls-files '*.md'`) and requires `title`; a skill's schema is Claude Code's and requires
`name`. Skills tolerate extra keys — the shipped ones carry `compatibility` and `license` — so one
added key satisfies both schemas. No `.github/frontmatter-exempt` entry: that file is for documents
carrying no frontmatter, and this one carries plenty.

### Requirement 3: #217's lane table matches the format

**This is a `gh issue edit`, not a commit.** The lane table lives in the issue body, so it is not in
the pull request and does not wait on it.

The report reads lane columns 3 and 4 by position and prints them as **Next** and **Waits on**:

| Schedule | Header |
| --- | --- |
| #206 | `Lane \| Item \| Next \| Waits on` |
| #232 | `Lane \| Item \| Next \| Waits on` |
| **#217** | `Lane \| Issue \| Repository \| Kind \| What closes it` |

So #217 renders `personal \| bug` under **Next** and **Waits on**. The report is faithful; #217 is
non-conforming, and lanes 13-19 were added in its shape — right locally, wrong against the format
#206 established.

Nineteen rows are reformatted to four columns. Repository and Kind are not lost: they move into the
Item cell, where the other two schedules carry that detail.

**The report reads by position without checking the header**, so a table with different columns is
confident nonsense rather than an error — the same family as #231 and #932. Named, not fixed here.

### Requirement 4: The machine is converged

Rule 5. A layer change is not done until the deploy has run and the skill resolves at
`~/.claude/skills/star-gh-report/SKILL.md`.

## Implementation Phases

### Phase 1: Commit the plan

- [x] This plan, before the change

### Phase 2: The skill

- [ ] `Home/common/.claude/skills/star-gh-report/SKILL.md` (Requirements 1, 2)

### Phase 2b: The schedule — outside the pull request

- [ ] #217's lane table reformatted to four columns, nothing lost (Requirement 3). A `gh issue edit`;
      it neither waits on the merge nor is carried by it

### Phase 3: Verify, then merge

- [ ] The frontmatter and PowerShell gates pass on this host
- [ ] The skill passes the frontmatter gate on its `title`, with no exemption added
- [ ] PR script written, analyzer-clean, shown, and handed over
- [ ] Codespell, Starlark and shell run on the pull request, and the merge gate blocks until every
      check reports pass

### Phase 4: Deploy

Not a handover — rule 5.

- [ ] writ's base clone pulled; `writ deploy common`
- [ ] `~/.claude/skills/star-gh-report/SKILL.md` resolves into the layer
- [ ] The report re-run against the reformatted #217: **Next** and **Waits on** read as themselves

## Related Documents

- [agent-rules.md](../../guides/agent-rules.md) — rule 3, and rule 1 which it depends on
- [#190](https://github.com/NobleFactor/noblefactor-ops/issues/190) — the report's output rendering, a different defect
- [#217](https://github.com/NobleFactor/noblefactor-ops/issues/217) — the schedule being reformatted
