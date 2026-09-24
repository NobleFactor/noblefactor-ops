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

    A reparse point counts as a link only when its tag says so. Three tags qualify:

      0xA000000C  IO_REPARSE_TAG_SYMLINK
      0xA0000003  IO_REPARSE_TAG_MOUNT_POINT   a junction
      0xA000001D  IO_REPARSE_TAG_LX_SYMLINK    a WSL symbolic link

    Everything else -- a OneDrive Files On-Demand placeholder, a deduplication stub, a Microsoft
    Store APPEXECLINK alias -- is not a link and is never removed, whatever else is true of it. The
    list is an allowlist because an unknown tag must fail closed: Microsoft adds tags, and a denylist
    would silently start deleting each new one.

    .NET models the first two and leaves LinkType and Target empty for the third, which is how
    56 dead links survived on one machine since 2026-01-01 (#241). A blank LinkType is not evidence
    that something is not a link; it is only evidence that .NET does not model it. So the tag is read
    for those, and a WSL link's target is decoded from the reparse payload.

    The Windows special folders under a profile, and node_modules, are skipped: they are not ours
    and are full of junctions that are not broken.

.PARAMETER Path
    The directory to walk. Defaults to the user's home.

.EXAMPLE
    Remove-BrokenLinks -WhatIf

    Lists every broken link under the home directory, with the target it resolved, and removes
    nothing.

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

###########
# Reparse points
###########

# The tags that mean "this is a link". Anything absent is left alone, however broken it looks.
$script:LinkTags = @{
    '0xa000000c' = 'symbolic link'
    '0xa0000003' = 'junction'
    '0xa000001d' = 'WSL symbolic link'
}

function Get-ReparsePointDetail {
    <#
    .SYNOPSIS
        The reparse tag, and for a WSL symbolic link its target. $null when the point cannot be read.

    .DESCRIPTION
        fsutil is the only reader available to a script: there is no managed API for the tag, and
        DeviceIoControl through P/Invoke is a heavier mechanism than one external call on the rare
        path. It is called only for entries .NET left blank, so an ordinary machine never calls it.

        The dump is fixed-width. Seven characters of offset and padding, then exactly 49 characters
        of hex field, then the ASCII column -- which is skipped by width rather than by pattern,
        because two ASCII characters can look exactly like a hex byte.
    #>
    [CmdletBinding()]
    [OutputType([psobject])]
    param(
        [Parameter(Mandatory)]
        [string]
        $LiteralPath
    )

    $output = & fsutil reparsepoint query $LiteralPath 2>&1

    if ($LASTEXITCODE -ne 0) {
        return $null
    }

    $text = $output | Out-String

    if ($text -notmatch 'Reparse Tag Value\s*:\s*(0x[0-9a-fA-F]+)') {
        return $null
    }

    $tag = $Matches[1].ToLowerInvariant()
    $target = $null

    if ($tag -eq '0xa000001d') {
        $bytes = foreach ($line in $output) {
            if ($line -match '^[0-9a-fA-F]{4}:' -and $line.Length -gt 7) {
                $field = $line.Substring(7)

                if ($field.Length -gt 49) {
                    $field = $field.Substring(0, 49)
                }

                foreach ($token in ($field -split '\s+')) {
                    if ($token -match '^[0-9a-fA-F]{2}$') {
                        [Convert]::ToByte($token, 16)
                    }
                }
            }
        }

        # Four bytes of version header, then the target as UTF-8.
        if ($bytes.Count -gt 4) {
            $target = [System.Text.Encoding]::UTF8.GetString($bytes[4..($bytes.Count - 1)])
        }
        else {
            $target = ''
        }
    }

    return [pscustomobject]@{
        Tag    = $tag
        Target = $target
    }
}

function Resolve-LinkTarget {
    <#
    .SYNOPSIS
        A link's target as an absolute path, resolved against the link's own directory when relative.
    #>
    [CmdletBinding()]
    [OutputType([string])]
    param(
        [Parameter(Mandatory)]
        [string]
        $LinkPath,

        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]
        $Target
    )

    if ([string]::IsNullOrEmpty($Target)) {
        return ''
    }

    if ([System.IO.Path]::IsPathRooted($Target)) {
        return $Target
    }

    return (Join-Path -Path (Split-Path -Path $LinkPath -Parent) -ChildPath $Target)
}

function Test-BrokenLink {
    <#
    .SYNOPSIS
        Whether one reparse point is a link whose target is gone. Emits the verdict and the target.

    .DESCRIPTION
        Nothing is emitted for a reparse point that is not a link, so a caller that receives nothing
        leaves the item alone. A link whose target exists is emitted with Broken false, so -Verbose
        can account for what was examined.
    #>
    [CmdletBinding()]
    [OutputType([psobject])]
    param(
        [Parameter(Mandatory)]
        [System.IO.FileSystemInfo]
        $Item
    )

    if ($Item.LinkType) {
        # .NET models this one. Its own resolver follows the chain, which the decoder cannot.
        $resolved = $Item.ResolveLinkTarget($true)

        return [pscustomobject]@{
            Item   = $Item
            Target = $Item.Target
            Kind   = $Item.LinkType
            Broken = -not $resolved.Exists
        }
    }

    $detail = Get-ReparsePointDetail -LiteralPath $Item.FullName

    if ($null -eq $detail -or -not $script:LinkTags.ContainsKey($detail.Tag)) {
        return
    }

    $target = Resolve-LinkTarget -LinkPath $Item.FullName -Target $detail.Target

    return [pscustomobject]@{
        Item   = $Item
        Target = $detail.Target
        Kind   = $script:LinkTags[$detail.Tag]
        Broken = [string]::IsNullOrEmpty($target) -or -not (Test-Path -LiteralPath $target)
    }
}

###########
# Main
###########

# Not ours, and full of junctions that resolve fine but are slow to walk.
$excluded = 'AppData', 'Application Data', 'Cookies', 'Local Settings', 'NetHood', 'PrintHood', 'Recent',
    'SendTo', 'Start Menu', 'Templates', 'node_modules' | ForEach-Object { [regex]::Escape((Join-Path $Path $_)) }
$excludedPattern = '^(' + ($excluded -join '|') + ')'

# A home directory always holds a few directories this user cannot list; they are not ours to fix
# and hold nothing we deploy, so the walk continues past them rather than stopping.
$examined = Get-ChildItem -LiteralPath $Path -Recurse -Force -Attributes ReparsePoint -ErrorAction SilentlyContinue |
    Where-Object FullName -NotMatch $excludedPattern |
    ForEach-Object { Test-BrokenLink -Item $_ }

$broken = @($examined | Where-Object Broken)
$removed = 0

foreach ($link in $broken) {
    $description = "$($link.Item.FullName) -> $($link.Target) [$($link.Kind)]"

    if ($PSCmdlet.ShouldProcess($description, 'Remove broken link')) {
        $link.Item | Remove-Item -Force
        Write-Verbose "Removed $description"
        $removed++
    }
}

Write-Information -InformationAction Continue "Removed $removed of $($broken.Count) broken link(s) under $Path."
