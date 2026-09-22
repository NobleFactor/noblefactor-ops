#!/usr/bin/env pwsh
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 Noble Factor. All rights reserved.

<#
.SYNOPSIS
    The PowerShell half of the quality gate: every tracked .ps1 against docs/guides/powershell-style-guidelines.md.

.DESCRIPTION
    Four checks per file, in the order a reader meets them:

      1. The first line is exactly `#!/usr/bin/env pwsh`, and no byte-order mark precedes it (guide section 1). A BOM
         parses, but it hides the shebang from the loader, so the file stops being runnable on Unix.
      2. The file parses.
      3. Every function, and the script itself, has [CmdletBinding()] and a param() block (guide section 2). An AST
         walk rather than a PSScriptAnalyzer custom rule, ruled 2026-09-22 (#218): the built-in rules do not cover it,
         and a custom-rule module is a heavier mechanism than one walk.
      4. PSScriptAnalyzer passes, with the organization's settings file.

    Every file is checked and every failure is reported, with its path and line, before the script exits non-zero
    once. A gate that stops at the first failure costs a CI round trip per defect.

    The file list is `git ls-files '*.ps1'`, so an untracked scratch file is not gated.

.PARAMETER Path
    Files to check, in place of the tracked list. For testing this script against planted defects.

.PARAMETER Settings
    The PSScriptAnalyzer settings file. Defaults to the repository's copy, which writ also deploys to
    ~/.config/PSScriptAnalyzer.

.EXAMPLE
    ./.github/scripts/Test-PowerShell.ps1

.EXAMPLE
    ./.github/scripts/Test-PowerShell.ps1 -Path ./scratch/Broken.ps1
#>

#Requires -Version 7.0

[CmdletBinding()]
param(
    [string[]]
    $Path,

    [string]
    $Settings = (Join-Path -Path $PSScriptRoot -ChildPath '..' -AdditionalChildPath '..', 'Home', 'common', '.config',
        'PSScriptAnalyzer', 'PSScriptAnalyzerSettings.psd1')
)

$ErrorActionPreference = 'Stop'

###########
# Helper functions
###########

function Get-TrackedScript {
    <#
    .SYNOPSIS
        Every tracked .ps1, as an absolute path.

    .DESCRIPTION
        Absolute, because git prints paths relative to the repository root while .NET's file APIs resolve a relative
        path against the process working directory, which PowerShell's own location does not change.
    #>
    [CmdletBinding()]
    [OutputType([string[]])]
    param()

    $root = git rev-parse --show-toplevel

    if ($LASTEXITCODE -ne 0) {
        throw "git rev-parse failed with exit code $LASTEXITCODE."
    }

    $tracked = git ls-files '*.ps1'

    if ($LASTEXITCODE -ne 0) {
        throw "git ls-files failed with exit code $LASTEXITCODE."
    }

    return [string[]] @($tracked | ForEach-Object { Join-Path -Path $root -ChildPath $_ })
}

function Test-Shebang {
    <#
    .SYNOPSIS
        The first line is the shebang, and no BOM precedes it. Returns the failures, if any.
    #>
    [CmdletBinding()]
    [OutputType([string[]])]
    param(
        [Parameter(Mandatory)]
        [string]
        $LiteralPath
    )

    $failures = @()
    $bytes = [System.IO.File]::ReadAllBytes($LiteralPath)

    if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
        $failures += 'line 1: a UTF-8 byte-order mark precedes the shebang, which hides it from the loader'
    }
    elseif ($bytes.Length -ge 2 -and (($bytes[0] -eq 0xFF -and $bytes[1] -eq 0xFE) -or
            ($bytes[0] -eq 0xFE -and $bytes[1] -eq 0xFF))) {
        $failures += 'line 1: the file is UTF-16; every text file is UTF-8 without a byte-order mark'
    }

    $first = (Get-Content -LiteralPath $LiteralPath -TotalCount 1 -Encoding utf8)

    if ($first -ne '#!/usr/bin/env pwsh') {
        $failures += "line 1: expected '#!/usr/bin/env pwsh', found '$first' (style guide section 1)"
    }

    return $failures
}

function Test-CmdletBinding {
    <#
    .SYNOPSIS
        Every function, and the script itself, has [CmdletBinding()] and a param() block. Returns the failures.
    #>
    [CmdletBinding()]
    [OutputType([string[]])]
    param(
        [Parameter(Mandatory)]
        [System.Management.Automation.Language.ScriptBlockAst]
        $Ast
    )

    $failures = @()

    # The script's own param() block. A script with no functions and no parameters is a statement list -- a profile
    # fragment, say -- and the guide asks for the binding there too.
    if ($null -eq $Ast.ParamBlock) {
        $failures += 'line 1: the script has no param() block (style guide section 2)'
    }
    elseif (-not ($Ast.ParamBlock.Attributes.TypeName.FullName -contains 'CmdletBinding')) {
        $failures += "line $($Ast.ParamBlock.Extent.StartLineNumber): the script's param() block has no [CmdletBinding()] (style guide section 2)"
    }

    foreach ($function in $Ast.FindAll({ $args[0] -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true)) {
        $parameters = $function.Body.ParamBlock

        if ($null -eq $parameters) {
            $failures += "line $($function.Extent.StartLineNumber): function '$($function.Name)' has no param() block (style guide section 2)"
            continue
        }

        if (-not ($parameters.Attributes.TypeName.FullName -contains 'CmdletBinding')) {
            $failures += "line $($function.Extent.StartLineNumber): function '$($function.Name)' has no [CmdletBinding()] (style guide section 2)"
        }
    }

    return $failures
}

function Test-Script {
    <#
    .SYNOPSIS
        One file through all four checks. Returns the failures, in the order they were found.
    #>
    [CmdletBinding()]
    [OutputType([string[]])]
    param(
        [Parameter(Mandatory)]
        [string]
        $LiteralPath,

        [Parameter(Mandatory)]
        [string]
        $SettingsPath
    )

    $failures = @()
    $failures += Test-Shebang -LiteralPath $LiteralPath

    $errors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($LiteralPath, [ref] $null, [ref] $errors)

    if ($errors.Count -gt 0) {
        # A file that does not parse cannot be walked or analysed; its parser errors are the whole report.
        return $failures + @($errors | ForEach-Object { "line $($_.Extent.StartLineNumber): $($_.Message)" })
    }

    $failures += Test-CmdletBinding -Ast $ast
    $failures += Invoke-ScriptAnalyzer -Path $LiteralPath -Settings $SettingsPath |
        ForEach-Object { "line $($_.Line): $($_.RuleName): $($_.Message)" }

    return $failures
}

###########
# Main
###########

if (-not (Test-Path -LiteralPath $Settings)) {
    throw "No PSScriptAnalyzer settings at $Settings."
}

if (-not (Get-Module -ListAvailable -Name PSScriptAnalyzer)) {
    throw 'PSScriptAnalyzer is not installed. Install-Module PSScriptAnalyzer -Scope CurrentUser'
}

$scripts = @(if ($Path) { $Path | ForEach-Object { (Resolve-Path -LiteralPath $_).Path } } else { Get-TrackedScript })
$failed = 0

foreach ($script in $scripts) {
    $failures = @(Test-Script -LiteralPath $script -SettingsPath $Settings)

    if ($failures.Count -eq 0) {
        continue
    }

    $failed++
    Write-Information -InformationAction Continue "FAIL $script"
    $failures | ForEach-Object { Write-Information -InformationAction Continue "  $_" }
}

Write-Information -InformationAction Continue "powershell: $($scripts.Count) checked, $failed with findings"

if ($failed -gt 0) {
    exit 1
}
