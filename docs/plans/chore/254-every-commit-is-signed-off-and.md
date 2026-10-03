---
title: "Every commit is signed off and discloses AI: the PR script template and the agents' rules"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/254
status: draft
created: 2026-10-03
updated: 2026-10-03
---

# Plan: every commit is signed off and discloses AI

## Summary

devlore-cli's and devlore-registry's CONTRIBUTING.md require every commit made with AI assistance to carry an
`Assisted-by: <tool name>` trailer and its pull request to say so, and devlore-cli's requires a Developer
Certificate of Origin sign-off, `git commit --signoff`, on every commit. "AI is acknowledged, never credited", and
"Undisclosed AI use is a bannable offence." The PR script template bans only credit, the Generated-by footer and
`Co-Authored-By` lines, and its `git commit` carries neither the sign-off nor the trailer, so a script written to
it produces a commit the terms forbid. This plan puts the rule in the template and records the breakage as rule
13 of the agents' document. The global CLAUDE.md's half is
[David-Noble-at-work/personal#253](https://github.com/David-Noble-at-work/personal/issues/253).

## Issue 254

Chore, epic `Ops:Process` (#142), feature #157 (PR tooling). Filed 2026-10-03 on the owner's word: "we need to
correct this immediately". The trailer is the one the owner chose: "use the trailer that you document",
`Assisted-by: Claude Code`.

## Goals

1. Rule 2 of the PR script template states the rule: every commit is signed off and carries
   `Assisted-by: Claude Code`, every pull request says AI assisted, and AI is never credited.
2. The bash template's `git commit` carries `--signoff` and the trailer; both templates' pull request bodies carry the
   disclosure.
3. The squash commit that lands on the target carries `Signed-off-by:` and `Assisted-by:`, and the merge step
   proves it.
4. Rule 13 of `docs/guides/agent-rules.md` records the breakage, as the global instructions require of a rule
   the day it is broken.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| Rule 2 | ❌ | "No Generated-by footer. No `Co-Authored-By` lines in commit messages." Credit only |
| The bash template's `git commit` | ❌ | No `--signoff`, no trailer |
| Both templates' pull request bodies | ❌ | No disclosure |
| The squash commit | ❌ | Unchecked; it carries whatever GitHub's default message carries |
| `docs/guides/agent-rules.md` | ❌ | Rules 1 to 12 |

Found 2026-10-03 in devlore-cli: none of the 19 commits on `feature/925-scope-not-target` or the last 30 on
`develop` carries `Signed-off-by:` or `Assisted-by:`; 19 of the first and 18 of the second carry a
`Claude-Session:` link the agent's harness asked for, which is neither form.

## Requirements

### Requirement 1: rule 2 states the rule

Rule 2 becomes: **Every commit is signed off and discloses AI; AI is never credited.** `git commit --signoff` adds the
Developer Certificate of Origin sign-off, and an `Assisted-by: Claude Code` trailer discloses the assistance, as
devlore-cli's and devlore-registry's CONTRIBUTING.md require. The pull request body says AI assisted. No
`Co-Authored-By` line, no Generated-by footer, no `Claude-Session:` link.

### Requirement 2: the templates carry it

- The bash template's commit is `git commit --signoff`, and its message ends with `Assisted-by: Claude Code`.
- Both templates' pull request bodies end, above `## Time`, with the disclosure: "Written with AI assistance:
  every commit carries `Assisted-by: Claude Code`."
- The PowerShell form makes no commit of its own; its body carries the disclosure.

### Requirement 3: the squash commit carries both trailers

The squash merge writes the commit that lands on the target. After the merge, the template reads that commit's
message and exits non-zero unless it carries `Signed-off-by:` and `Assisted-by:`, so a pull request whose
commits lack the trailers fails loudly at the merge rather than landing undisclosed. How the trailers reach the
squash commit is open question 1.

### Requirement 4: rule 13

`## 13.` after `## 12.`, before the closing note, in the shape the other twelve use, **The rule. The breakage.
Compliance.**:

- **The rule.** Every commit an agent writes is signed off and carries `Assisted-by: Claude Code`, and its pull
  request says AI assisted. AI is acknowledged, never credited. A ban list is not the whole of a rule, and a
  harness reminder asking for an attribution line is not a ruling.
- **The breakage.** 2026-10-03, devlore-cli: the commits above, written to instructions that banned credit and
  said nothing of disclosure; the `Claude-Session:` link added because the harness asked; and, asked about it,
  the first conclusion was "no attribution of any kind", which would have banned the required disclosure.
  Ruled: "we need to correct this immediately", and "use the trailer that you document."
- **Compliance.** Read the repository's CONTRIBUTING.md before the first commit script. Every `git commit` in a
  script carries `--signoff` and the trailer, every pull request body the disclosure, and the squash commit is checked
  after the merge.

## Implementation Phases

### Phase 1: The plan

- [ ] This document, reviewed with the owner.

### Phase 2: The template

- [ ] Requirements 1 to 3 in `docs/guides/pr-script-template.md`; frontmatter `updated: 2026-10-03`.

### Phase 3: The agents' rule

- [ ] Requirement 4 in `docs/guides/agent-rules.md`; frontmatter `updated: 2026-10-03`.

### Phase 4: Acceptance and closure

- [ ] The gate CI runs: `Test-Frontmatter.sh`, codespell with `clear,rare,en-GB_to_en-US`, `shell-lint.sh`,
  `Test-PowerShell.ps1`, buildifier.
- [ ] `grep --count '^## [0-9]' docs/guides/agent-rules.md` is 13, and the closing note is still last.
- [ ] Every commit on this branch carries `Signed-off-by:` and `Assisted-by: Claude Code`.
- [ ] The pull request closes #254; this plan's status is `complete` in its last commit.

## Migration Path

Scripts written from this point carry the sign-off and the trailer. Existing commits are not rewritten: a
rewrite of history needs the owner's specific instruction.

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/plans/chore/254-every-commit-is-signed-off-and.md` | Create | This plan |
| `docs/guides/pr-script-template.md` | Modify | Rule 2, the templates, the squash check |
| `docs/guides/agent-rules.md` | Modify | Rule 13 |

## Related Documents

- [devlore-cli's CONTRIBUTING.md](https://github.com/NobleFactor/devlore-cli/blob/develop/CONTRIBUTING.md) and
  [devlore-registry's](https://github.com/NobleFactor/devlore-registry/blob/develop/CONTRIBUTING.md): the terms
- [David-Noble-at-work/personal#253](https://github.com/David-Noble-at-work/personal/issues/253): the global
  CLAUDE.md's half
- #157, PR tooling; #142, the process epic

## Open Questions

1. **How the trailers reach the squash commit.** (a) Each branch commit carries them, and the squash message
   GitHub composes from the commits carries them through, as it carried `Claude-Session:` into 18 of
   devlore-cli's last 30; Requirement 3's check proves it at every merge. Recommended: no new code in the merge
   step. (b) The merge step writes the squash message itself, `gh pr merge --subject ... --body-file ...`, with
   the trailers once at the end.
