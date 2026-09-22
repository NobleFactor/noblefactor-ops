---
title: "The git-* process commands fail closed when Declare-BashScript is missing"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/214
status: complete
created: 2026-09-22
updated: 2026-09-22
---

# Plan: The git-* process commands fail closed

## Issue 214

Lane 4 of #217. On 2026-09-21 DANOBLE-WD11-3 ran `git close-branch` with its `Declare-BashScript`
sibling dangling. The `source` failed, the script fell through into its option loop with `$@`
unparsed, and the `*)` arm called `error`, which the missing helper was to define. `error: command
not found` neither exits nor shifts, so the loop spun until killed. `git-open-branch` and
`git-reset-branch` have the same shape.

## Goals

1. `git-open-branch`, `git-close-branch`, and `git-reset-branch` say once that the helper is missing,
   name the directory they looked in, and exit 72.
2. The helper's own man page gives the guarded form, so new scripts copy it.
3. The personal sweep is filed: personal#201.

## Requirements

### Requirement 1: The guard

Ahead of the `source` in each command:

```bash
[[ -r "$(dirname "$0")/Declare-BashScript" ]] || {
    printf '%s: cannot find Declare-BashScript in %s\n' "${0##*/}" "$(dirname "$0")" >&2
    exit 72 # EX_OSFILE
}
```

Checked first rather than `source ... || exit`: a command on the left of `||` runs with errexit
suppressed, and that would include the whole helper. Exit 72, `EX_OSFILE`, rather than the issue's
suggested 70: 72 is the sysexits code for a missing critical file, and `git-open-branch` already
uses 70 for a branch that could not be created.

### Requirement 2: Documentation

Each command's man page lists exit 72. `Declare-BashScript(1) § USAGE` shows the guard above the
`source` and says why it is not `|| exit`.

## Implementation Phases

### Phase 1: Commit

- [x] The guard in the three commands (Requirement 1)
- [x] Exit 72 in `git-open-branch.1`, `git-close-branch.1`, `git-reset-branch.1`; the guarded form
      in `Declare-BashScript.1` (Requirement 2)
- [x] This plan, then the change, in that order

### Phase 2: Verify

- [x] Each command alone in an empty directory exits 72 at once with the message; the develop copies,
      under a 5 s timeout, loop until killed (exit 124) -- 2026-09-22
- [x] With the helper beside them: `git-open-branch -h` and `git-close-branch -h` exit 0;
      `git-open-branch 216 --dry-run` derives `chore/216-...`; `git-reset-branch -h` exits 78, its
      existing `require_nix` refusal on Windows -- 2026-09-22
- [x] `shfmt -d -i 4 -ci` and `shellcheck -x --severity=warning -P ~/.local/bin`, the CI flags, clean
      on danoble-mbp-a.local -- 2026-09-22
- [x] `mandoc -Tlint -W warning` on the four pages: only the `.TH` date warning every page in the
      repository carries; the USAGE block renders as written -- 2026-09-22

## Files

| File | Action | Purpose |
| --- | --- | --- |
| `Home/common/.local/bin/git-open-branch` | Modify | The guard |
| `Home/common/.local/bin/git-close-branch` | Modify | The guard |
| `Home/common/.local/bin/git-reset-branch` | Modify | The guard |
| `Home/common/.local/share/man/man1/git-open-branch.1` | Modify | Exit 72 |
| `Home/common/.local/share/man/man1/git-close-branch.1` | Modify | Exit 72 |
| `Home/common/.local/share/man/man1/git-reset-branch.1` | Modify | Exit 72 |
| `Home/common/.local/share/man/man1/Declare-BashScript.1` | Modify | The guarded usage form |

## Out of Scope

- Personal's roughly sixty consumers of the helper: personal#201.
- A defense inside `Declare-BashScript` against `error` being undefined. The guard makes that state
  unreachable in a guarded script, and the helper cannot guard against its own absence.
