---
title: "PowerShell Style Guidelines"
type: Guide
status: Approved
---

# PowerShell Style Guidelines

These guidelines define the NobleFactor style for PowerShell scripts and modules. They parallel the Go Style
Guidelines while following PowerShell's command, parameter, and pipeline model.

## 1. File Layout (MANDATORY)

Every PowerShell script, in exact order:

1. **Copyright header** — SPDX identifier and copyright notice
2. **Comment-based help** — `.SYNOPSIS`, `.DESCRIPTION`, parameters, and examples when useful
3. **`#Requires` declarations** — PowerShell 7.0 or later and required modules
4. **Script-level `[CmdletBinding()]` and `param()`** — before executable statements
5. **Script configuration** — preference variables and constants
6. **Helper functions** — in a `Helper functions` region immediately after `$ErrorActionPreference`
7. **Main operation** — the executable workflow, after all helper definitions

A script's main operation should read as an orchestration of named actions. Move implementation detail into a helper
function when doing so makes the main operation easier to follow. Functions are defined before the main operation so
that every helper is available when the script begins executing.

PowerShell 7 or later is the default supported runtime. PowerShell 5.1 is not supported. A script that deliberately
supports PowerShell 5.1 must state that support explicitly in its comment-based help and use a corresponding
`#Requires -Version 5.1` declaration.

## 2. Cmdlet Binding

Every function definition uses `[CmdletBinding()]`, including parameterless functions:

```powershell
function Get-RepositoryName {
    [CmdletBinding()]
    param()

    ...
}
```

The `[CmdletBinding()]` attribute is the first statement in the function body and is followed immediately by its
`param()` block. Use advanced-function behavior consistently rather than making some functions advanced and others
not.

The script itself also uses `[CmdletBinding()]` before its `param()` block.

## 3. Function Organization

Functions are grouped by responsibility and placed after the main operation. Helper functions are ordered so that
higher-level operations appear before the primitives they use when that improves reading order; otherwise use
alphabetical order within a responsibility group.

Function names use approved PowerShell verb-noun pairs. Use `Get-` for queries, `Test-` for predicates, `ConvertTo-`
for transformations, `Invoke-` for operations, and `Require-` for required prerequisites.

Functions should have one responsibility. Prefer a named function over duplicated command sequences or an inline
scriptblock whose behavior is not obvious at the call site.

## 4. Control Structures and Braces

Use BSD brace style. The opening brace is on the statement line, and `else`, `elseif`, `catch`, and `finally` begin
on the next line with their opening brace on the same line:

```powershell
if ($condition) {
    ...
}
else {
    ...
}
```

Do not use one-line `if` expressions or one-line control structures. Always use a block, even when it contains one
statement:

```powershell
if ($value) {
    return $value
}
```

Put one blank line before and one blank line after every control structure. An `if`/`elseif`/`else` chain is one
control structure, so there is no blank line between its branches:

```powershell
$branch = Get-Branch

if (-not $branch) {
    $branch = 'detached'
}

if ($Issue) {
    $title = Get-IssueTitle $Issue
}
else {
    $title = Get-PlanTitle $branch
}

$sessionName = "$repository | $branch"
```

Apply the same spacing to `foreach`, `for`, `while`, `do`, `switch`, `try`, and `trap`. Do not introduce multiple
consecutive blank lines. A scriptblock used as a pipeline callback follows the same BSD brace placement, but blank
lines should separate the surrounding pipeline logically rather than interrupting a short callback.

## 5. Parameters and Naming

- Use approved PowerShell verb-noun names for functions.
- Use PascalCase for function and parameter names.
- Use descriptive names; do not use one-letter variables except for conventional short-lived indexes.
- Use `[Parameter()]` and validation attributes when the contract requires them.
- Put each significant parameter on its own declaration when attributes or defaults make the contract clearer.
- Use `ValueFromRemainingArguments` only for intentional pass-through arguments.
- Preserve native command argument boundaries with arrays rather than building unquoted command strings.

## 6. Error Handling

- Set `$ErrorActionPreference = 'Stop'` when the script requires fail-fast behavior.
- Check the result of an external command immediately after invocation.
- Check `$LASTEXITCODE` immediately after native commands that expose it.
- Throw an actionable message that identifies the failed operation and relevant input.
- Use `-ErrorAction SilentlyContinue` only when absence is an expected branch, and handle that branch explicitly.
- Return `$null` only when absence is part of the function contract; do not silently discard unexpected failures.
- Validate user input at the parameter boundary with validation attributes or immediately after parsing it.

## 7. External Commands and Working Directories

Prefer PowerShell APIs and cmdlets for filesystem and process operations. External dependencies must be explicit and
minimal. Keep working-directory changes local to the operation that requires them.

A launcher that invokes Bash must not acquire general Bash dependencies. Bash dependencies are limited to commands whose
names begin with `git-`; PowerShell launchers should use native PowerShell and Windows commands instead.

## 8. Formatting

- Use four spaces for indentation.
- Use BSD braces consistently.
- Keep lines at or below 120 columns when practical; break long pipelines at natural command boundaries.
- Put a blank line after a function's `param()` block before its body.
- Use blank lines to separate logical blocks, with exactly one blank line around control structures.
- Do not add a blank line before a short function's final `return`.
- Keep comments short and explain decisions, constraints, or non-obvious behavior.
- Do not add comments that merely narrate the following statement.
- Avoid one-line expressions when a block makes control flow clearer.
- Preserve argument quoting and array structure; do not use `Invoke-Expression` for command construction.

## 9. Comments and Help

Script-level comment-based help documents the command contract. Function comments are required for exported module
functions and recommended for non-obvious private helpers.

Comments should explain why the code has a non-obvious shape, especially when it works around native command parsing,
Windows Terminal behavior, transcript naming, or path semantics. Keep implementation narration out of comments.

## 10. Validation

Every change is followed by a focused check before further edits:

- Parse the script with PowerShell's language parser.
- Verify every function has `[CmdletBinding()]` and an immediate `param()` block.
- Check brace placement and blank-line rules when formatting changes are involved.
- Run the narrowest available behavior check without launching interactive workflows unnecessarily.
- Fix every parser, lint, or validation issue as discovered; do not pass by warnings or errors.

A launcher should be validated without invoking its external interactive session unless the test specifically
requires it.
