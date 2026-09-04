---
title: "Commit the PowerShell style guidelines, which every Windows launcher already conforms to"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/137
status: complete
created: 2026-09-03
updated: 2026-09-03
---

# Plan: Commit the PowerShell style guidelines

## Summary

`docs/guides/powershell-style-guidelines.md` is Approved in its own frontmatter and is what
`Start-Claude.ps1` was written against, yet it has never been committed. Untracked, it cannot be
linked from an issue or read from another machine. This commits it, unchanged, in the worktree whose
pull request carries the process revisions it belongs beside.

## Goals

1. Make the guide a committed file, so issues can link it and other machines can read it.
2. Change nothing in it. Its two internal contradictions are recorded, not resolved.

## Requirements

### Requirement 1: The file, as it is

Committed verbatim. The document is the deliverable; there is no design behind it to link.

### Requirement 2: Known contradictions, left open

Recorded on David-Noble-at-work/personal#160's plan as open questions, and not ruled on here:

- Section 1 places helper functions before the main operation; section 3 places them after it.
- Section 3 mandates the `Require-` verb, which is not an approved PowerShell verb, so
  PSScriptAnalyzer raises `PSUseApprovedVerbs` on every use.

## Implementation Phases

### Phase 1: Commit — complete

- [x] Copy the untracked file into this worktree and commit it

## Issue 137

[Commit the PowerShell style guidelines, which every Windows launcher already conforms
to](https://github.com/NobleFactor/noblefactor-ops/issues/137)

The whole plan serves this one issue. It is resolved in the #135 worktree because the guide belongs
beside the process revisions that pull request carries.

## Related Documents

- Issue #137
- [135-unit-of-work-is-a-session.md](135-unit-of-work-is-a-session.md) — the plan sharing this worktree
- David-Noble-at-work/personal#160 — the launcher written against this guide, and where the open
  questions live
