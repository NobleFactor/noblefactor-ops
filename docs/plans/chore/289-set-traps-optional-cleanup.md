---
title: "The helper's shellcheck notes are gone, and it stops when a tool it needs is missing"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/289
status: active
created: 2026-10-09
updated: 2026-10-09
---

# Plan: The helper's shellcheck notes and the tools it needs

## Issues 289 and 290

Lanes 63 and 65 of NobleFactor/devlore-cli#916, one pull request, ahead of lane 62 (NobleFactor/devlore-cli#1037).
The owner's rulings, all 2026-10-09:

- #289. Offered one directive on `Set-Traps` in the helper, one above each of personal's five calls, or leaving
  shellcheck's note: "Fix it in the helper". Offered this issue taking two more notes, in the helper's test: "go with
  1".
- #290. Told that macOS's `getopt` misparses the helper's call and exits 0: "we stop if we have the wrong version of
  bash. I'm either in or out based on the bash version number." Then: "and if any other tool we require is missing.
  same deal." Offered `command -v` or `hash` for the check: "use hash. file the issue."
- The order. Offered this pull request first or lane 62's: "1 followed by 2 as you recommended".
- No parser. Offered parsing options in pure bash so `install.sh` could share the helper's: "I do not want to
  maintain an argument parser without a drop-dead good reason and I'm not sure that macOS is a good enough reason."

## Goals

1. Shellcheck, at every severity, notes nothing at a call of `Set-Traps` with no cleanup, nor in the helper's test.
2. The helper stops when the `getopt` first on PATH is not GNU's, in or out, as it stops for a bash older than 5.3.
3. A script can stop before it starts when a tool it needs is missing: `require_command`, which checks with `hash`.
4. The man page and the helper's test cover all of it.

## Current State

Measured 2026-10-09 on Danoble-MBP-A, shellcheck 0.11.0, `-x --severity=style`, `-P` at the helper's directory.

| What | Today |
| --- | --- |
| `Set-Traps` called with no cleanup | SC2119 (info) at each call: personal's `Start-MacSleep`, `Start-MacWakeup`, `Test-MacUp`, `Unmount-ExternalPhysicalDisks` and `New-RcloneMountUnit`, each at line 16, and this repository's `tests/Test-DeclareBashScript:52`. The gates check warnings and errors only. |
| The helper's test | SC2016 (info) at lines 100 and 105, where `run_case` is handed programs for a child bash in single quotes. |
| `getopt` | The helper parses with GNU `getopt --long` (`Declare-BashScript:241`). GNU's `getopt --test` exits 4; macOS's `/usr/bin/getopt` exits 0, and given `Backup-TimeMachine`'s call it returns `-- Backup-TimeMachine -o h --long help,bandwidth-limit: -- --bandwidth-limit 10M extra` with exit 0: the script's loop stops at the first `--`, every option is dropped, and the rest become arguments. On macOS a newer bash and GNU getopt are separate packages, and Homebrew's `gnu-getopt` is not put on PATH, so the bash check vouches for nothing here. Git for Windows' bash on danoble-wd11-3 has GNU's, util-linux 2.40.2. |
| A script's own tools | Found missing only where first run: exit 127 and the handler's report, after what came before has run. |
| The helper's other tools | `dirname`, `basename`, `uname -s` and a bare `xargs`, in their POSIX forms. `man` is only tried by `usage`, which falls back to the synopsis. |
| `hash` | Tested on bash 5.3.20 and 3.2.57: it reports a tool missing from PATH, accepts a function of that name, and forgets what it found when PATH is assigned; on 5.3.20, it finds a stand-in program first on PATH. |
| Plan 268 | Still `active`: its CI box and its closure box wait on its own merge, which happened as #288 on 2026-10-09. |

## Requirements

### Requirement 1: `Set-Traps`'s cleanup is optional (#289)

`# shellcheck disable=SC2120` directly above `function Set-Traps {`, saying why: the cleanup is optional, and a
script with nothing to undo calls it bare. Shellcheck raises SC2120 at a function that reads its arguments when no
call passes any, and SC2119 at each such call; disabling the first at the definition silences both, which was tested
in one file. That it does so through a sourced file is what this requirement proves (phase 2).

### Requirement 2: the test's quoted programs (#289)

`# shellcheck disable=SC2016` above each of the two `run_case` calls, saying why: the program is for the child bash,
and its `$` must reach it unexpanded.

### Requirement 3: GNU getopt, in or out (#290)

Beside the bash check, and before `set -o errexit`: `getopt --test` must exit 4. Otherwise the helper says, in the
bash check's form, that it requires GNU getopt and that the `getopt` first on PATH is not it, names where to get it,
MacPorts' `util-linux` or Homebrew's `gnu-getopt` with its directory put on PATH, and exits 78 (`EX_CONFIG`). It
searches for no other copy.

### Requirement 4: `require_command` (#290)

`require_command <name>...` looks each name up with `hash` and names every one not found in a single `error`, exiting
69 (`EX_UNAVAILABLE`, the helper's code for a missing dependency). It sits with the other `require_*` functions.
Proposed: which scripts call it, and with what, goes to the scripting standards (#266), not here.

### Requirement 5: documentation

The man page names the GNU getopt requirement beside bash's, with its exit status, and documents `require_command`
among the provided functions.

### Requirement 6: tests

`tests/Test-DeclareBashScript` gains three cases, and `run_case` a prelude the child runs before it sources the
helper:

1. a `getopt` that is not GNU's, a function standing in for it, since a function runs ahead of any program of that name
   on PATH, so the test writes no file: the helper exits 78 with its message
2. `require_command` with a tool that is missing: exit 69, naming it
3. `require_command` with tools that are all found: the program carries on

### Requirement 7: function documentation

The owner, 2026-10-09, reviewing this branch: "I see that the comments in Declare-BashScript DO NOT conform to my
standards. As in go, all function documentation should: start with a one line description (see the go style guide);
continue with zero or more paragraphs of elaboration (newline separated); end with a description of Parameters and
Returns (see the go style guide)." Recorded for every bash script on #267.

Every function in the helper, 12 of them, of which 3 had documentation and none in this form, and the test's
`run_case` take the Go guide's § 4 form, written with `#` as `git-close-branch` writes it: a one-line summary that
begins with the function's name, zero or more paragraphs separated by a blank `#` line, then `# Parameters:`, each
`` `$1` `` and so on, or `none`, and `# Returns:`, the exit status, what goes to stdout, or that the function ends
the script. Both sections are always present, as the Go guide has them on every method.

### Requirement 8: plan 268's open boxes

CI passed on #288, which merged on 2026-10-09, and `git close-branch` removed its branch and worktree. Plan 268's CI
box and its closure box are ticked. Its last box, lanes 61 and 62, records lane 61 merged as
David-Noble-at-work/personal#267 and stays open for lane 62, so plan 268 stays `active`.

## Implementation Phases

### Phase 1: The plan

- [x] This plan, committed on `chore/289-set-traps-optional-cleanup` as 9bf3354 and reviewed with the owner; its
  question settled by an earlier ruling. Approved 2026-10-09: "approved. let's go."

### Phase 2: The shellcheck notes (#289)

- [x] Requirements 1 and 2. Done 2026-10-09: shellcheck at every severity, with `-P` at this worktree's helper, is
  clean on the helper, its test, and personal's five scripts as merged in David-Noble-at-work/personal#267, where
  `develop`'s helper still draws SC2119 at `Start-MacSleep`. The directive in the sourced file silences every call.

### Phase 3: The tools the helper needs (#290)

- [x] Requirements 3 to 6. Done 2026-10-09: `getopt --test` must exit 4, or the helper exits 78 before anything else
  runs; `require_command` checks with `hash` and exits 69, naming each missing tool; the man page carries both; and the
  three new cases pass in `make check` with the fourteen before them. The getopt case's stand-in is a function, so the
  test writes no file. Two of the man page's older lines, past 120 columns, are wrapped; it renders the same.

### Phase 4: Function documentation

- [x] Requirement 7. Done 2026-10-09: all 12 of the helper's functions and the test's `run_case` carry the form,
  checked by a script that wants a one-line summary beginning with the function's name, `# Parameters:` before
  `# Returns:`, and every line within 120 columns. shfmt, shellcheck at every severity and `make check` are clean.

### Phase 5: Plan 268

- [ ] Requirement 8.

### Phase 6: Verification

- [ ] `make check` clean on this Mac, and CI green on the pull request.
- [ ] Shellcheck at every severity, against this worktree's helper: nothing at personal's five calls as merged in
  David-Noble-at-work/personal#267, nothing in `tests/Test-DeclareBashScript`.
- [ ] `getopt --test` on danoble-ud24-1 and in Git for Windows' bash on danoble-wd11-3, each read before the change
  lands, since the check stops every script where it fails. 2026-10-09: danoble-wd11-3 exits 4 (util-linux 2.40.2,
  bash 5.3.15); danoble-ud24-1 refused the connection, "Host key verification failed".
- [ ] The deployed check, as lane 60 ran it: the branch checked out in the base clone and deployed, the helper's
  tests run against the deployed helper, and `--help` for the consumers whose code before their option loop runs
  nothing.

### Phase 7: Acceptance and closure

- [ ] The pull request, the merge, and `git close-branch`; lanes 63 and 65 marked on NobleFactor/devlore-cli#916.

## Files

| File | Action | Purpose |
| --- | --- | --- |
| `Home/common/.local/bin/Declare-BashScript` | Modify | Requirements 1, 3, 4 and 7 |
| `Home/common/.local/share/man/man1/Declare-BashScript.1` | Modify | Requirement 5 |
| `tests/Test-DeclareBashScript` | Modify | Requirements 2, 6 and 7 |
| `docs/plans/chore/268-declare-bashscript-traps.md` | Modify | Requirement 8 |
| `docs/plans/chore/289-set-traps-optional-cleanup.md` | Create | This plan |

## Out of Scope

- `install.sh`'s narration, both forms of its options, and the helper's handler: lanes 62 and 64,
  NobleFactor/devlore-cli#1037 and #1038, after this pull request.
- Which scripts call `require_command`: the scripting standards, #266.
- `Set-Environment`'s three defects, found writing its documentation: #291, lane 66, the next noblefactor-ops pull
  request. The owner: "log and address the Set-Environment tweak in the next pr."
- Every other bash function's documentation, 3 of 218 in the form today: #292, for the scripting standards.

## Open Questions

1. **The handler's report under bash 3.2. Settled before it was asked, by the owner's ruling of 2026-10-09: "we stop
   if we have the wrong version of bash. I'm either in or out based on the bash version number."** Under 3.2 the
   helper stops before its handler exists, so there is no 3.2 report to fix; the question was withdrawn after the
   owner's "we're going over the same territory." Lane 61 held the same: "it should not be bound to bash 3.2."
