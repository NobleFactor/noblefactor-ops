---
title: "A blank LinkType hides a dead link from the tool built to remove it"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/241
status: active
created: 2026-09-24
updated: 2026-09-24
---

# Plan: a blank LinkType hides a dead link

## Issue 241

`Remove-BrokenLinks.ps1` reported `Removed 0 of 0 broken link(s)` on a machine carrying **56 dead
links**, every one of them present since 2026-01-01.

Line 56 is a truthiness test:

```powershell
Where-Object LinkType |
```

Windows leaves `LinkType` and `Target` **empty** for a WSL-format symbolic link — reparse tag
`0xA000001D`, `IO_REPARSE_TAG_LX_SYMLINK` — so all 56 are discarded before line 57 asks whether the
target exists. They are as dead as a link gets: `ResolveLinkTarget($true)` returns `$null`, `Length`
is 0, and a read throws *"The file cannot be accessed by the system."*

Ruled 2026-09-24: **a blank link is broken.**

## Requirements

### Requirement 1: a reparse point is a link only when its tag says so

The filter that hides these links is also load-bearing. `ResolveLinkTarget` returns `$null` for any
reparse point .NET does not model — OneDrive Files On-Demand placeholders, deduplication stubs,
Microsoft Store `APPEXECLINK` aliases — so simply deleting line 56 turns the script into a shredder
on a machine that has them. This script ships from the base layer to every Windows machine and cannot
assume this one's absence of OneDrive.

So classify by tag, as an **allowlist**:

| Tag | Name | Is a link |
| --- | --- | --- |
| `0xA000000C` | `IO_REPARSE_TAG_SYMLINK` | yes |
| `0xA0000003` | `IO_REPARSE_TAG_MOUNT_POINT` | yes (junction) |
| `0xA000001D` | `IO_REPARSE_TAG_LX_SYMLINK` | yes |
| anything else | — | **no, and never touched** |

A denylist would have to enumerate every tag Microsoft has ever shipped and stay correct as they add
more. An allowlist fails closed: an unknown tag is left alone.

### Requirement 2: the tag is read only when it has to be

`LinkType` is populated for ordinary symlinks and junctions, which is the overwhelming majority.
The tag is read only for entries where `LinkType` is blank — 56 of 180 reparse points on this
machine, and zero on a machine that never ran WSL. The common path keeps its current cost.

### Requirement 3: `-WhatIf` stops lying

`ShouldProcess` interpolates `$link.Target`, which is the empty string for precisely the links this
issue is about, so the dry run would print `~/.local/bin/foo -> ` and say nothing useful. The decoded
target is carried alongside the link and printed.

### Requirement 4: the decoded target decides, not the tag alone

An `0xA000001D` link whose target **does** exist is not broken and is not removed. The payload is
decoded — a 4-byte version header followed by the UTF-8 target — and resolved against the link's own
directory, exactly as line 57 does for links .NET understands.

## Implementation phases

### Phase 1: the plan lands — status `complete`

- [x] Written
- [x] Committed before any other change on this branch

### Phase 2: the fix — status `draft`

- [ ] `Get-ReparseTag` — the tag for one path, or `$null` when it cannot be read
- [ ] `Get-LinuxSymlinkTarget` — decode the `0xA000001D` payload to its target string
- [ ] `Test-BrokenLink` — one predicate: is this reparse point a link, and is it broken
- [ ] The pipeline at line 54 rewritten to use it, with the allowlist
- [ ] `ShouldProcess` prints the decoded target
- [ ] `gofmt`-equivalent: the file passes `Test-PowerShell.ps1`

**Files**: `Home/common.Windows/.local/bin/Remove-BrokenLinks.ps1` — modify.

### Phase 3: proof — status `draft`

The machine is the fixture: 56 dead WSL links, 111 working symlinks, 13 junctions.

- [ ] `-WhatIf` over `$HOME` lists **56** and removes nothing
- [ ] Each listed entry names a decoded target, not a blank
- [ ] A planted **good** symlink is not listed
- [ ] A planted **broken** ordinary symlink is listed — the existing path still works
- [ ] A planted non-link reparse point is not listed
- [ ] The run is clean on a machine with no WSL links

### Phase 4: handover — status `draft`

- [ ] `go-noblefactor-ops.ps1` regenerated for this branch
- [ ] Shown with a summary, and the one command handed over
- [ ] The owner runs it

### Phase 5: after the merge — status `draft`

Rule 5: a layer change is finished when the machine is converged.

- [ ] `writ deploy common`
- [ ] The deployed script removes the 56, with the owner watching the `-WhatIf` list first
- [ ] Issue closed
- [ ] This document set to `complete`

## Related documents

- [#195](https://github.com/NobleFactor/noblefactor-ops/issues/195) — where the script came from
- [#218](https://github.com/NobleFactor/noblefactor-ops/issues/218) — the PowerShell gate this file must pass
