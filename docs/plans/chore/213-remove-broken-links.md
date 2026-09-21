---
title: "Remove-BrokenLinks.ps1 ships from the base layer's common.Windows"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/213
status: chartered
created: 2026-09-21
updated: 2026-09-21
---

# Plan: Remove-BrokenLinks.ps1 ships from the base

## Issue 213

Lane 2 of #217. A deploy that moves files in a writ layer leaves the old links behind, pointing at
where the files were; on 2026-09-21 DANOBLE-WD11-3 carried 110 of them. The only tool for it was a
function buried in personal's Tuckr-era `Install-WindowsUserConfiguration.ps1`, unreachable
without running that installer and defective on PowerShell 7, where `Test-Path` on a dangling link
is true. Ruled 2026-09-21: extract it to the base as a standalone script, pure PowerShell.

The script was written and used under the personal#191 session: `-WhatIf` listed 44 links, the
real run removed them, and the home directory reported zero. It is the base layer's first
`common.Windows` entry and its first PowerShell file.

## Goals

1. `Home/common.Windows/.local/bin/Remove-BrokenLinks.ps1` is committed, conforming to
   `docs/guides/powershell-style-guidelines.md`, and deploys wherever the base does on Windows.
2. Nothing else in the base changes in this lane.

## Requirements

### Requirement 1: The script

`#!/usr/bin/env pwsh`, the Apache-2.0 header, comment-based help, `#Requires -Version 7.0`,
`[CmdletBinding(SupportsShouldProcess)]` with a `-Path` parameter defaulting to `$HOME`,
`$ErrorActionPreference = 'Stop'`, a Windows guard that throws, no functions. Detection is
`Get-ChildItem -Recurse -Force -Attributes ReparsePoint`, `LinkType`, and
`ResolveLinkTarget($true).Exists`; removal is `Remove-Item` under `ShouldProcess`, so `-WhatIf`
lists and `-Verbose` names each link. The Windows special folders under a profile, and
`node_modules`, are skipped.

Accepted limit: links Git Bash's `ln -s` creates carry reparse tag `0xA000001D`, which no Windows
API resolves; PowerShell returns an empty `Target` for them and the script does not see them.
Everything writ deploys is an NTFS symbolic link and is seen. Ruled 2026-09-21: no P/Invoke for
the WSL kind; "that is crap."

### Requirement 2: Placement

`common` deploys unconditionally and depends on nothing outside itself; a PowerShell script has no
sibling to source, so `common.Windows` is right, and the `.Windows` selector keeps it off Unix
boxes. This is the rule of `development-process.md § Where a script lives`.

## Implementation Phases

### Phase 1: Commit

- [x] The script at `Home/common.Windows/.local/bin/Remove-BrokenLinks.ps1` (Requirement 1)
- [x] This plan, then the script, in that order

### Phase 2: Verify

- [x] Parser: 0 errors; script-level `[CmdletBinding()]` and `param()`; no functions; shebang on
      line 1; no line over 120 columns -- 2026-09-21
- [x] `-WhatIf` against `~/.local` on DANOBLE-WD11-3 lists exactly the links `find -xtype l`
      reports for NTFS links; the real run on 2026-09-21 removed 44 and left zero -- verified with
      a second `-WhatIf`
- [x] The base's gate: frontmatter passes locally (51 checked, 0 errors); codespell runs on the PR;
      shell-lint and buildifier are untouched, since no shell or Starlark file changes
- [x] PSScriptAnalyzer 1.25.0, default rules: 0 findings -- 2026-09-21
- [ ] After merge and deploy: `Get-Command Remove-BrokenLinks` resolves into the base on
      DANOBLE-WD11-3, and `Remove-BrokenLinks -WhatIf` lists the eight links personal#191's merge
      left dangling

## Files

| File | Action | Purpose |
| --- | --- | --- |
| `Home/common.Windows/.local/bin/Remove-BrokenLinks.ps1` | Create | The script |

## Out of Scope

- Retiring the function from personal's `Install-WindowsUserConfiguration.ps1`. A personal lane
  once this ships.
- A PowerShell step in the base's CI gate. Ruled 2026-09-21: its own lane, after #216 settles the
  guide's shebang rule, so the gate enforces the finished rule. That is lane 10 of #217. Until it
  lands this file is validated by hand and by PSScriptAnalyzer 1.25.0, which reports 0 findings.

## Open Questions

- [x] **Does the base's CI gate gain a PowerShell step?** Yes, in its own lane after #216 -- ruled
      2026-09-21, option (b).
