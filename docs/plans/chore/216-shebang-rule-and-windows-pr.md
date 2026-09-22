---
title: "PowerShell scripts start with a shebang, and PR scripts on Windows are PowerShell"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/216
status: complete
created: 2026-09-22
updated: 2026-09-22
---

# Plan: The shebang rule, and the Windows PR script

## Issue 216

Lane 10 of #217. Ruled 2026-09-21: "all of our pwsh scripts should include shebang lines". Also ruled that day: PR
scripts on Windows are PowerShell, not bash. Neither is written down. `powershell-style-guidelines.md` §1 lists seven
layout items, starting with the copyright header, and §10 doesn't check the first line. `pr-script-template.md` is bash
only.

**The reason has moved since the issue was filed.** It gave Git Bash as the shell people type into on Windows. Since
2026-09-22 the `!` prompt there is pwsh (`defaultShell: powershell`), and a pwsh session runs a `.ps1` directly. The
rule holds for two reasons that don't depend on that:

- **On macOS and Linux,** an executable `.ps1` with `#!/usr/bin/env pwsh` runs directly: `./Script.ps1`, from any
  shell. Without it, the calling shell parses the file itself and fails at `<#` or `[CmdletBinding()]`.
- **From any bash, Git Bash included,** the same is true. Personal's lane 5 showed it:
  `syntax error near unexpected token 'PositionalBinding'` from `main`, and a clean run with the shebang.

## Current State

Read 2026-09-22 on `develop` at `1d075c4`.

- The base has one `.ps1`, `Home/common.Windows/.local/bin/Remove-BrokenLinks.ps1`, and it starts with
  `#!/usr/bin/env pwsh`.
- `pr-script-template.md` is stale in three places beyond the missing Windows form:
  - Its description and opening say "the bash PR script" and name `~/Workspace/NobleFactor/go`, which the
    `go-<repo>` naming in the global instructions replaced.
  - Rule 9 and the closing block still make every PR end with `star gh issues report`. `development-process.md`
    says: "A pull request ends with it when one is asked for ... Ruled 2026-09-12, retiring the standing rule that
    every pull request print one." The template and the process doc contradict each other.
- Three PowerShell PR scripts exist and have been run this week: `go-personal.ps1`, `go-noblefactor-ops.ps1` and
  `go-devlore-cli.ps1`. Their shape is the one the template should record.

## Goals

1. The style guide requires the shebang, says why, and checks for it.
2. The PR script template has a Windows form, and it matches the scripts that work.
3. The template agrees with `development-process.md` about the end-of-PR report.

## Requirements

### Requirement 1: `powershell-style-guidelines.md`

- **§1 gains item 0,** above the copyright header: `#!/usr/bin/env pwsh` on line 1, and the file executable
  (`git update-index --chmod=+x` on Windows) when it's a script that's run. That covers dot-sourced files too, so the
  rule is "every `.ps1`" with no exception, as ruled for personal#193. Why: stated as above.
- **§10 gains a first-line check,** beside the parser check: line 1 is `#!/usr/bin/env pwsh`, with no BOM ahead of it.
  A BOM hides the shebang from the loader.
- **§6 says `-ErrorAction Ignore` for an expected absence,** not `SilentlyContinue`. Found while implementing: §6 named
  `SilentlyContinue`, which still records to `$Error`. That is the cause of the four entries personal#199 cleared
  from every profile load.

### Requirement 2: `pr-script-template.md`, the Windows form

A section, "On Windows: PowerShell", with a skeleton of `go-<repo>.ps1` and the rules that differ from bash:

- The file is `go-<repo>.ps1`, one level above the repository, to the PowerShell guide: shebang, help, `#Requires`,
  `[CmdletBinding()]`/`param()`, `$ErrorActionPreference = 'Stop'`, helpers, then main.
- **`$LASTEXITCODE` after every native command,** through one helper (`Assert-NativeSuccess`), because
  `$ErrorActionPreference` doesn't cover native exit codes and there is no `set -e`.
- **Single-quoted here-strings** for the PR body and the jq program, so PowerShell expands nothing in them.
- **The body through a file,** `gh pr create --body-file`, and the `## Time` section written with
  `gh api -X PATCH ... -F body=@<file>`.
- **Clean-up from the main clone:** `Set-Location` there before `git close-branch`, because Windows won't delete a
  directory that is a process's current directory. No `git pull` after it: `git close-branch` fast-forwards the main
  worktree itself (see Phase 2).
- **No PATH changes in a script.** Run from a pwsh whose `git` is `Git\cmd\git.exe`, the launcher, which gives git's
  children the `sh` and `bash` they need. The raw `Git\clangarm64\bin\git.exe` has neither, and an HTTPS push or
  `git close-branch` exits 128 without a message.
- **The owner runs the script.** The agent writes it, shows it, and hands over one command.

The bash template stays as the Unix form, unchanged except below.

### Requirement 3: `pr-script-template.md`, the stale parts

- The description and opening name both forms and the `go-<repo>` location.
- Rule 9 and the closing `star gh issues report` block say what `development-process.md` says: the report ends a PR
  when one is asked for, scoped to what the PR served.

## Implementation Phases

### Phase 1: Commit

- [x] This plan, first
- [x] Requirement 1, with section 6's `-ErrorAction Ignore` -- 2026-09-22
- [x] Requirements 2 and 3 -- 2026-09-22

### Phase 2: Verify

- [x] The frontmatter gate (`./.github/scripts/Test-Frontmatter.sh`) passes locally, 56 checked and 0 errors; codespell and the rest run in CI -- 2026-09-22
- [x] The Windows skeleton, extracted into a scratch file, parses, starts with the shebang, holds only ASCII, and reports 0 PSScriptAnalyzer findings -- 2026-09-22
- [x] Every rule in the Windows form is one the three working scripts follow -- 2026-09-22. Two claims were checked
      against source, not carried over. First, `git close-branch` ends by fast-forwarding the main worktree to
      `origin/<target>` (its closing `merge --ff-only`), so the skeleton has no `git pull` after it, and
      `go-devlore-cli.ps1`'s pull was redundant: it printed `Already up to date`. Second, the reason given for the
      REST body edit is only that every Windows script has used it; the old comment that `gh pr edit` trips on the
      Projects (classic) field was never verified, so it isn't repeated

## Out of Scope

- The global instructions (`~/.claude/CLAUDE.md`), which still say "a self-contained bash script". They live in
  personal, `Home/common/.claude/CLAUDE.md`, and change in the Personal PR that follows this one.
- The CI gate for PowerShell: lane 11, #218.
- The Personal sweep of `.ps1` shebangs: done in personal#193.

## Open Questions

- [x] **The global instructions' "bash script" wording:** change it, ruled 2026-09-22. The file is personal's
      `Home/common/.claude/CLAUDE.md`, so it goes in a Personal PR right after this one.
