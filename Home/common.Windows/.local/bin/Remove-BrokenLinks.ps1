#!/usr/bin/env pwsh
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 Noble Factor. All rights reserved.

<#
.SYNOPSIS
    Removes symbolic links under a directory whose targets no longer exist.

.DESCRIPTION
    Walks the directory recursively, including hidden entries. For every symbolic link or junction,
    reads the target from the link, resolves it against the link's own directory, and removes the
    link if the target does not exist. Linked directories are not descended into.

    The Windows special folders under a profile, and node_modules, are skipped: they are not ours
    and are full of junctions that are not broken.

.PARAMETER Path
    The directory to walk. Defaults to the user's home.

.EXAMPLE
    Remove-BrokenLinks -WhatIf

    Lists every broken link under the home directory and removes nothing.

.EXAMPLE
    Remove-BrokenLinks -Path ~/.local -Verbose

    Removes broken links under ~/.local, naming each one.
#>

#Requires -Version 7.0

[CmdletBinding(SupportsShouldProcess)]
param(
    [Parameter(Position = 0)]
    [ValidateScript({ Test-Path -LiteralPath $_ -PathType Container })]
    [string]
    $Path = $HOME
)

$ErrorActionPreference = 'Stop'

if (-not $IsWindows) {
    throw 'Remove-BrokenLinks targets Windows. On Unix, find -xtype l -delete does this.'
}

# Not ours, and full of junctions that resolve fine but are slow to walk.
$excluded = 'AppData', 'Application Data', 'Cookies', 'Local Settings', 'NetHood', 'PrintHood', 'Recent',
    'SendTo', 'Start Menu', 'Templates', 'node_modules' | ForEach-Object { [regex]::Escape((Join-Path $Path $_)) }
$excludedPattern = '^(' + ($excluded -join '|') + ')'

# A home directory always holds a few directories this user cannot list; they are not ours to fix
# and hold nothing we deploy, so the walk continues past them rather than stopping.
$broken = Get-ChildItem -LiteralPath $Path -Recurse -Force -Attributes ReparsePoint -ErrorAction SilentlyContinue |
    Where-Object FullName -NotMatch $excludedPattern |
    Where-Object LinkType |
    Where-Object { -not $_.ResolveLinkTarget($true).Exists }

$removed = 0

foreach ($link in $broken) {
    if ($PSCmdlet.ShouldProcess("$($link.FullName) -> $($link.Target)", 'Remove broken link')) {
        $link | Remove-Item -Force
        Write-Verbose "Removed $($link.FullName) -> $($link.Target)"
        $removed++
    }
}

Write-Information -InformationAction Continue "Removed $removed of $(@($broken).Count) broken link(s) under $Path."
