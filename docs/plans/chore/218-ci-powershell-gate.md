---
title: "The CI gate gains a PowerShell step"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/218
status: active
created: 2026-09-22
updated: 2026-09-22
---

# Plan: The CI gate gains a PowerShell step

## Issue 218

Lane 11 of #217, the last noblefactor-ops lane. Ruled 2026-09-21 on #213: the gate gains a PowerShell step after #216
settles the shebang rule, so the gate enforces the finished rule. #216 merged as #225.

## Current State

Read 2026-09-22 on `develop` at `693d4e4`.

- `ci.yaml`'s `quality-gate` job runs four steps: Frontmatter, Spelling, Shell, Starlark. Nothing reads a `.ps1`. The
  job name is fixed by organization ruleset 12426847.
- The repository has one `.ps1`: `Home/common.Windows/.local/bin/Remove-BrokenLinks.ps1`. PSScriptAnalyzer 1.25.0
  reports 0 findings on it, and it starts with the shebang.
- The style guide, since #225, requires the shebang on line 1 with no BOM (§1), `[CmdletBinding()]` with an immediate
  `param()` on every function (§2), and the same first-line and structure checks at validation time (§10).
- `ubuntu-latest` ships pwsh, so the step needs no PowerShell install. PSScriptAnalyzer comes from the PowerShell
  Gallery.
- The settings file's location was ruled on this issue, 2026-09-22: `Home/common/.config/PSScriptAnalyzer/
  PSScriptAnalyzerSettings.psd1`, which writ deploys to `~/.config/PSScriptAnalyzer/PSScriptAnalyzerSettings.psd1`,
  and which CI reads from the repository. Not the repository root, which the issue text predates.

## Goals

1. A `.ps1` that breaks the guide fails the gate, in this repository and in any repository that adopts the script.
2. CI, a developer's machine and VS Code run one rule set, from one file.
3. The gate is reproducible: a pinned analyzer version, as the Shell and Starlark steps pin theirs.

## Requirements

### Requirement 1: `PSScriptAnalyzerSettings.psd1`

At `Home/common/.config/PSScriptAnalyzer/PSScriptAnalyzerSettings.psd1`: the built-in rules, with
`Severity = @('Error', 'Warning', 'Information')`, which is the level personal#199 cleaned the profiles to. Any
excluded rule carries a comment saying why. The file is data, so it deploys with `common` and needs no selector.

### Requirement 2: `.github/scripts/Test-PowerShell.ps1`

Takes the files from `git ls-files '*.ps1'`, so untracked scratch files are not gated, and for each:

1. **The first line.** Bytes 1-2 are `#!`, no BOM, and line 1 is exactly `#!/usr/bin/env pwsh` (guide §1).
2. **The parser.** `[Parser]::ParseFile`, and any error fails.
3. **The structure.** Every `FunctionDefinitionAst` has a `[CmdletBinding()]` attribute and a `param()` block, and the
   script itself has both (§2). This is an AST walk in the script, not a PSScriptAnalyzer custom rule: the built-in
   `PSUseCmdletBinding` rules don't cover it, and a custom-rule module is a heavier mechanism than one walk.
4. **The analyzer.** `Invoke-ScriptAnalyzer -Settings <the file above>`, failing on any finding.

It reports every failure with its file and line, and exits non-zero once, rather than stopping at the first.
`#Requires -Version 7.0`, the shebang, `[CmdletBinding()]`, `param()` -- it obeys the guide it enforces.

### Requirement 3: The `ci.yaml` step

A `PowerShell` step in `quality-gate`, after Shell:

```yaml
      - name: PowerShell
        env:
          PSSCRIPTANALYZER_VERSION: 1.25.0
        shell: pwsh
        run: |
          Install-Module PSScriptAnalyzer -RequiredVersion $env:PSSCRIPTANALYZER_VERSION -Scope CurrentUser -Force
          ./.github/scripts/Test-PowerShell.ps1
```

The version is pinned, as the Shell and Starlark steps pin theirs: an analyzer that gains a rule overnight is a gate
that fails on someone else's commit. The Gallery serves modules over HTTPS with no published digest, so the pin is the
version, not a hash.

### Requirement 4: `pr-script-template.md`

The CI entry-point table gains the command for `noblefactor-ops`, so a pre-flight runs what CI runs.

## Implementation Phases

### Phase 1: Commit

- [x] This plan, first
- [x] Requirements 1 and 2 -- 2026-09-22
- [x] Requirements 3 and 4 -- 2026-09-22

### Phase 2: Verify before merge

- [x] `./.github/scripts/Test-PowerShell.ps1` passes locally: 2 checked, 0 with findings -- 2026-09-22
- [x] It fails, with the right message and exit 1, on each planted defect: no shebang (`expected '#!/usr/bin/env
      pwsh'`), a UTF-8 BOM, a UTF-16 file, a parse error, a function without `[CmdletBinding()]`, and trailing
      whitespace through the analyzer -- 2026-09-22
- [x] The script gates itself, and caught one finding in itself on the first run: `Get-TrackedScript` returned
      `Object[]` against a declared `string[]`. Fixed, then clean. It also found a real bug in its first draft:
      `git ls-files` prints repository-relative paths, which .NET's file APIs resolve against the process working
      directory rather than PowerShell's location, so the paths are made absolute from `git rev-parse --show-toplevel`
      -- 2026-09-22
- [ ] CI's PowerShell step passes on the PR

## Out of Scope

- **The blank-line rules of guide §4.** They need a formatter's model of the file, not an AST walk, and a
  half-implementation would reject code the guide allows. If they are to be enforced, that is its own issue.
- Adopting the script in personal or devlore-cli. Each has its own gate; personal's `.ps1` files pass the analyzer
  already (#199), and adopting this script there is a lane on a later schedule.

## Open Questions

- [x] **Severity floor:** Information and above, ruled 2026-09-22.
- [x] **The §2 structural check:** an AST walk in the script, ruled 2026-09-22. A custom rule stays available if editor
      feedback proves worth it.
