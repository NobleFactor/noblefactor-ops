---
title: "Rules that exist only because an agent broke them get a document of their own"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/185
status: in-progress
created: 2026-09-07
updated: 2026-09-07
---

# Plan: Rules that exist only because an agent broke them get a document of their own

## Summary

The process documents are written for engineers, and some rules would insult one: that a command
from a tool you are building must be built and installed before you run it; that the organization's
own report is how you look up its issues. Those rules exist because a coding agent broke them, and
they need a place the agents' instruction files point at and no engineer has to read. Ruled
2026-09-07: "put that into the process documents in a place reserved for LLMs. I would not embarrass a
human being by stating such an obvious thing." One new guide, one README row, no change to the
engineers' documents beyond a pointer.

## Goals

1. **One document, audience `Claude Code, Codex`**, holding only rules whose sole audience is an agent,
   each with the date it was broken and the words that ruled it.
2. **The engineers' documents stay the engineers'.** `development-process.md` and `pr-script-template.md`
   gain a one-line pointer at most.
3. **The instruction files can point at it.** The README's standards table names it; the deployed
   `~/.claude/CLAUDE.md` pointer is the personal layer's change and is filed there.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| Agent-only rules | in a session's memory files and in the user's patience | five so far, all 2026-09-04 to 2026-09-07 |
| `pr-script-template.md` | audience `Engineers, Claude Code` | the one document that addresses agents, about one thing |
| `development-process.md` | audience `Engineers` | correct; must not absorb these |
| README standards table | six rows | gains a seventh |

## Implementation Phases

### Phase 1: the guide

- [x] `docs/guides/agent-rules.md`, `type: Process`, `audience: Claude Code, Codex`, `status: Approved`,
      with six rules and the day each was broken (a sixth arrived while the plan waited: a layer change
      is finished when the machine is converged, not when the PR merges):
      1. tools we build are verified (fetch, build hash against develop HEAD, `make install`, said in the message) before a command is handed over
      2. rulings lead the guides and the binary; the ruled form is stated and the interim named
      3. issue questions go through `star gh issues report` and `audit`
      4. a generated list is proven with a dry run before it is handed over
      5. lead with the answer
- [x] Each rule in three parts: the rule, the breakage that produced it, how to comply
- [x] **Acceptance:** the frontmatter gate and codespell pass; nothing in it is a rule an engineer needs

### Phase 2: the pointers

- [x] README "The standards CLAUDE.md points at": a row for the guide, labelled as the agents' document
- [x] `pr-script-template.md`: its pre-flight section gains one sentence pointing at rule 1
- [x] Filed in personal as David-Noble-at-work/personal#175: `Home/common/.claude/CLAUDE.md` (deployed as
      `~/.claude/CLAUDE.md`) and the Codex mirror point at the guide — the personal-layer change
- [ ] **Acceptance:** `development-process.md` unchanged; the epic report at the end of the PR

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/guides/agent-rules.md` | Create | the agents' document |
| `README.md` | Modify | the table row |
| `docs/guides/pr-script-template.md` | Modify | one sentence |

## Decisions

- **A separate document, not a section.** A section in an engineers' guide is read by engineers. The
  audience field is the boundary.
- **Every rule carries its breakage.** The date and the words are what keep the rule from reading as
  condescension to a future reader, human or agent: it happened.
- **Nothing here duplicates the global instructions.** `CLAUDE.md` already carries the process; this
  holds what CLAUDE.md assumes and an agent still got wrong.

## Related Documents

- Issue #185 — this chore; feature #156, epic #142
- #183 — rule 2's breakage; personal#172 — rule 4's
- `docs/guides/pr-script-template.md` — the existing agent-facing document

## Open Questions

None.
