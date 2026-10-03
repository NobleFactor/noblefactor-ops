---
title: "Every pull request is signed off and discloses AI: the PR script template and the agents' rules"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/254
status: draft
created: 2026-10-03
updated: 2026-10-03
---

# Plan: every pull request is signed off and discloses AI

## Summary

devlore-cli's and devlore-registry's CONTRIBUTING.md require AI assistance to be disclosed, with an
`Assisted-by: <tool name>` trailer and in the pull request, and devlore-cli's requires a Developer Certificate of
Origin sign-off, `Signed-off-by:`. "AI is acknowledged, never credited", and "Undisclosed AI use is a bannable
offence." The PR script template bans only credit, the Generated-by footer and `Co-Authored-By` lines, and says
nothing of either, so a pull request made to it lands commits the terms forbid. Ruled 2026-10-03, the pull request
carries them, not the commits: its description says AI assisted and ends with both trailers, and the squash merge
writes that description into the commit that lands on the target. This plan puts the rule in the template and
records the breakage as rule 13 of the agents' document. The global CLAUDE.md's half is
[David-Noble-at-work/personal#253](https://github.com/David-Noble-at-work/personal/issues/253).

## Issue 254

Chore, epic `Ops:Process` (#142), feature #157 (PR tooling). Filed 2026-10-03 on the owner's word: "we need to
correct this immediately". The owner's rulings the same day:

1. **The trailer**: "use the trailer that you document", `Assisted-by: Claude Code`.
2. **The pull request carries it, not the commits**: "I want the prs, not the commits to carry these messages."
   Of the two ways this plan first offered, it is the second: the merge step writes the squash message itself.

## Goals

1. Rule 2 of the PR script template states the rule: every pull request's description says AI assisted and ends
   with `Signed-off-by:` and `Assisted-by: Claude Code`, the squash commit takes its message from it, commits on
   the branch carry no trailer, and AI is never credited.
2. Both templates' pull request bodies carry the disclosure and the trailers.
3. Both templates' merge steps write the squash message from the description and prove that the commit that
   landed carries both trailers.
4. Rule 13 of `docs/guides/agent-rules.md` records the breakage, as the global instructions require of a rule
   the day it is broken.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| Rule 2 | ❌ | "No Generated-by footer. No `Co-Authored-By` lines in commit messages." Credit only |
| Both templates' pull request bodies | ❌ | No disclosure, no trailers |
| Both templates' merge steps | ❌ | `gh pr merge --squash --admin`: GitHub composes the squash message |
| `docs/guides/agent-rules.md` | ❌ | Rules 1 to 12 |

Found 2026-10-03 in devlore-cli: none of the 19 commits on `feature/925-scope-not-target` or the last 30 on
`develop` carries `Signed-off-by:` or `Assisted-by:`; 19 of the first and 18 of the second carry a
`Claude-Session:` link the agent's harness asked for, which is neither form.

## Requirements

### Requirement 1: rule 2 states the rule

Rule 2 becomes: **Every pull request is signed off and discloses AI; AI is never credited.** The pull request's
description says AI assisted and ends with two trailers, `Signed-off-by: <name> <email>` and
`Assisted-by: Claude Code`, as devlore-cli's and devlore-registry's CONTRIBUTING.md require. The squash merge
writes the description into the commit that lands on the target, so the target's history carries both. Commits
on the branch carry no trailer. No `Co-Authored-By` line, no Generated-by footer, no `Claude-Session:` link.

### Requirement 2: the pull request bodies

Both templates' bodies end, above `## Time`, with the disclosure and the trailers as their last paragraph:

```text
Written with AI assistance.

Signed-off-by: <name> <email>
Assisted-by: Claude Code
```

The sign-off is filled from `git config user.name` and `git config user.email` when the script runs, never
typed: the bash form appends it after the quoted heredoc, and the PowerShell form builds it the same way.

### Requirement 3: the merge step writes the squash message

The bash form merges with `gh pr merge "${pr_number}" --squash --admin --subject "<title> (#${pr_number})"
--body-file <file>`, the file holding the description up to `## Time`, so the trailers end the message. The
PowerShell form does the same. After the final pull of the target, the template reads the landed commit's
message and exits non-zero unless it carries `Signed-off-by:` and `Assisted-by:`.

### Requirement 4: rule 13

`## 13.` after `## 12.`, before the closing note, in the shape the other twelve use, **The rule. The breakage.
Compliance.**:

- **The rule.** Every pull request an agent opens says AI assisted and ends its description with
  `Signed-off-by:` and `Assisted-by: Claude Code`, and the squash merge writes them into the commit that lands.
  AI is acknowledged, never credited. A ban list is not the whole of a rule, and a harness reminder asking for an
  attribution line is not a ruling.
- **The breakage.** 2026-10-03, devlore-cli: the commits above, written to instructions that banned credit and
  said nothing of disclosure; the `Claude-Session:` link added because the harness asked; and, asked about it,
  the first conclusion was "no attribution of any kind", which would have banned the required disclosure.
  Ruled: "we need to correct this immediately", "use the trailer that you document", and "I want the prs, not
  the commits to carry these messages."
- **Compliance.** Read the repository's CONTRIBUTING.md before the first PR script. The description carries the
  disclosure and both trailers, the merge step writes the squash message from it, and the landed commit is
  checked for both after the merge.

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
- [ ] This pull request's description carries the disclosure and both trailers, and the squash commit that lands
  on `develop` carries both.
- [ ] The pull request closes #254; this plan's status is `complete` in its last commit.

## Migration Path

Pull requests opened from this point carry the disclosure and the trailers. This plan's first commit, 7694104,
carries them in the commit itself, made before the second ruling. Existing commits are not rewritten: a rewrite
of history needs the owner's specific instruction.

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/plans/chore/254-every-commit-is-signed-off-and.md` | Create | This plan |
| `docs/guides/pr-script-template.md` | Modify | Rule 2, the pull request bodies, the merge step |
| `docs/guides/agent-rules.md` | Modify | Rule 13 |

## Related Documents

- [devlore-cli's CONTRIBUTING.md](https://github.com/NobleFactor/devlore-cli/blob/develop/CONTRIBUTING.md) and
  [devlore-registry's](https://github.com/NobleFactor/devlore-registry/blob/develop/CONTRIBUTING.md): the terms
- [David-Noble-at-work/personal#253](https://github.com/David-Noble-at-work/personal/issues/253): the global
  CLAUDE.md's half
- #157, PR tooling; #142, the process epic

## Open Questions

None. The one this plan first carried, how the trailers reach the squash commit, is ruling 2 above.
