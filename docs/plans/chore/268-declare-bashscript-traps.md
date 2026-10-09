---
title: "on_error_or_interrupt moves into Declare-BashScript: one handler, its defects fixed, each script stating its traps and cleanup"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/268
status: active
created: 2026-10-08
updated: 2026-10-08
---

# Plan: Declare-BashScript traps

## Issue 268

Lane 60 of NobleFactor/devlore-cli#916, ahead of lane 49's phase 17 (David-Noble-at-work/personal#260), whose
startup hook needs a trap. The owner, 2026-10-08:

- "Ask yourself the question: does it make sense to move on_error_or_interrupt into Declare-BashScript where
  specific scripts would add cleanup code?" Then: "This would work more like arg handling in that user code would be
  required."
- "We will commit your outstanding work and then take the time to update Declare-BashScript with trap logic. We will
  accept the trap logic when all bash scripts in base, team, and main that have trap logic using the
  Declarre-BashScript trap logic. We will then proceed with Start-Claude work."
- Asked whether this takes the next lane and whether personal's scripts that trap get a chore of their own: "yes",
  "yes". Of the five bash files that trap without the helper, the owner: "We have a DevLore-cli install.sh script
  that could embed Declare-BashScript but should not source it." Offered taking all five now, each the way its
  repository allows: "All five now".

So the acceptance spans three pull requests: this one (lane 60), personal's fourteen scripts
(David-Noble-at-work/personal#266, lane 61), and devlore-cli's three (NobleFactor/devlore-cli#1037, lane 62).

## Goals

1. The helper defines the handler. A script states its traps in its own code, as it writes its option loop.
2. The handler's three defects are gone: a signal stops the script, a failure inside a function is reported, and a
   failure that does not end the script is not reported as if it had.
3. The report is the helper's own narration, `error`'s `[name] [✘] ...`.
4. A script's cleanup runs on every way out: a failure, a signal, an `error` call and a normal end.
5. The base's one script that traps, `scripts/setup-ground-zero.sh`, sources the helper and traps through it.

## Current State

Surveyed 2026-10-08 on Danoble-MBP-A, here-documents excluded. `Declare-BashScript` defines no trap. Its first
statement is `set -o errexit -o nounset -o pipefail`.

| Who traps | How |
| --- | --- |
| Five personal scripts | `on_error_or_interrupt` on `ERR SIGINT SIGTERM`, copied four ways: file and line (`Start-MacSleep`, `Start-MacWakeup`); status only, under `# shellcheck disable=SC2329` (`Test-MacUp`); status only (`Unmount-ExternalPhysicalDisks`); a UTC timestamp first (`New-RcloneMountUnit`) |
| Eight more personal scripts | EXIT traps of their own: `cleanup`, `cleanup_on_failure`, `after_local_backup`, `after_cloud_backup`, inline `rm` |
| Five bash files that do not source the helper | `install.sh`, `packaging/macports/generate-portfile.sh` and `scripts/Test-InstallScript.sh` in devlore-cli; `scripts/setup-ground-zero.sh` here; `Install-NFBuildTools` in personal |
| The other 57 scripts that source the helper | no trap |

**The copied handler fails three ways.** Measured 2026-10-08 with bash 5.3.15, in `bash -c` probes outside the
helper, each signal sent by a child of the script rather than by a terminal:

1. A script sent SIGTERM or SIGINT while a command runs prints `exited due to error 0`, runs on to its next line,
   and exits 0. The handler returns, so the trap swallows the signal.
2. A failure inside a function exits 1 with no message: without `set -o errtrace`, an ERR trap is not inherited by
   functions.
3. With `errtrace` on, the trap also runs for a failure inside `$(...)`, and for one the script tolerates under
   `set +o errexit`, while the script carries on.

**A candidate handler, probed the same way.** The helper sets `errexit`, `errtrace`, `nounset` and `pipefail` and
`shopt -s inherit_errexit`; the handler takes the event's name:

```bash
function on_error_or_interrupt {
    local status=$? event=$1
    case $event in
        ERR)
            [[ $- == *e* ]] || return 0              # a failure the script tolerates
            ((BASH_SUBSHELL == 0)) || exit "$status" # the shell that started the subshell reports it
            error "$status" "exited with status ${status} at line ${BASH_LINENO[0]}: ${BASH_COMMAND}"
            ;;
        INT | TERM)
            error 0 "interrupted by SIG${event}"
            trap - "$event"
            kill -s "$event" "$$"
            ;;
    esac
}
```

| Case | Result |
| --- | --- |
| `false` at top level | one report, its line and command, status 1 |
| a failure inside a function | one report, the failing command inside the function, its status |
| a failure inside `x="$(...)"` | one report, the assignment's line, the inner command's status |
| `false` under `set +o errexit` | no report; the script carries on |
| a failure in an `if` or before `\|\|` | no report |
| `error 64 ...` | the message, the EXIT trap's cleanup, status 64 |
| SIGINT | the report, the EXIT trap's cleanup, status 130 |
| SIGTERM | the report and status 143, **but the EXIT trap's cleanup never ran**: bash dies of the re-raised SIGTERM without running it |

The last row is question 2.

## Requirements

Each is a proposal until the owner rules on the open questions below.

### Requirement 1: the helper's handler

`Declare-BashScript` first refuses bash older than 5.3 with `EX_CONFIG`, as `require_*` refuses a platform, before any
statement older bash would reject (question 5). It sets `errtrace` beside `errexit`, `nounset` and `pipefail`, and
`shopt -s inherit_errexit` (question 4), and defines `on_error_or_interrupt` as above, with question 2's signal ending
and question 3's signals. It reports through `error`, so a crash reads like every other failure the scripts narrate.
A signal ends the script with 128 plus its number, through `error`, so the EXIT trap's cleanup runs and the caller
sees 129, 130 or 143 (question 2):

```bash
        HUP | INT | TERM)
            error $((128 + $(kill -l "$event"))) "interrupted by SIG${event}"
            ;;
```

It also defines `Set-Traps`, the one call a script makes (question 1):

```bash
function Set-Traps {
    trap 'on_error_or_interrupt ERR' ERR
    trap 'on_error_or_interrupt HUP' HUP
    trap 'on_error_or_interrupt INT' INT
    trap 'on_error_or_interrupt TERM' TERM
    if [[ -n ${1:-} ]]; then trap "$1" EXIT; fi
}
```

### Requirement 2: the code each script writes

A script that traps calls `Set-Traps`, naming its cleanup function when it has something to undo (question 1):

```bash
function cleanup {
    [[ -z "${download_dir}" ]] || rm -rf "${download_dir}"
}

Set-Traps cleanup
```

The function is defined before the call: an early exit with the EXIT trap naming an undefined function fails with
"command not found". `cleanup` reads the exit status from `$?`, so a script that undoes only on failure says so in its
own `cleanup`. The call follows the function it names and comes before the arguments are parsed (question 8). No
script sets these traps itself, nor defines an `on_error_or_interrupt` of its own.

### Requirement 3: documentation

`Declare-BashScript.1` gains a TRAPS section: the handler, the code a script writes, what is reported and when, the
exit statuses, and the cleanup protocol. The rest of the page's omissions are #286's.

### Requirement 4: the base's script

`scripts/setup-ground-zero.sh` sources the copy in this repository, `../Home/common/.local/bin/Declare-BashScript`,
under the guard and the `# shellcheck source=Declare-BashScript` directive, and takes the helper's whole pattern: its
own `info`, `success`, `warn` and `error` (which exits 1) give way to `note`, `success` and `error $EX_*`; its option
loop reads `$script_arguments`; its `trap 'rm -f "$AZURE_OUTPUT"' EXIT` becomes a `cleanup` under Requirement 2. Its
two siblings, `setup-azure-swa.sh` and `setup-github-repo.sh`, set no trap and are #287's.

### Requirement 5: tests

A committed test script, `tests/Test-DeclareBashScript`, shows each of the three defects, and every row of the
candidate's table, failing against today's copied handler and passing against the helper's (question 6). It sources
the helper as Requirement 4's script does, and runs from the `Makefile`, not from `.github` (question 7).

### Requirement 6: the `Makefile`

A root `Makefile`, modeled on devlore-cli's (question 7). `make test` runs every script under `tests/`. `make check`
runs every gate CI runs (frontmatter, spelling, shell-lint, PowerShell, Starlark) and then `test`. CI keeps installing
the gates' tools and calls `make check`, so a local run and CI are one command.

## Implementation Phases

### Phase 1: The plan

- [x] This plan, committed on `chore/268-declare-bashscript-traps` and reviewed with the owner; the open questions
  ruled. Approved 2026-10-08: "The plan is approved. Let's go".

### Phase 2: The handler

- [x] Requirements 1 and 3. 2026-10-08 on Danoble-MBP-A, bash 5.3.15, sourcing this worktree's helper from `bash -c`
  probes: a failure at top level, inside a function, inside `x="$(...)"` and inside `x="$(false; echo hi)"` is
  reported once with its line and command and exits with its status; a failure under `set +o errexit`, in an `if`
  test or before `||` is not reported; SIGHUP, SIGINT and SIGTERM are reported and exit 129, 130 and 143; the cleanup
  ran after a failure, each signal, `error 64` and a normal end, reading the status from `$?`; macOS's `/bin/bash`
  3.2.57 is refused with 78 and a message. Two behaviors of bash itself, written into the man page: a failing
  pipeline is reported with its last command's text, and a function called as an `if` test runs past its own
  failure. The helper's `getopt` call, 131 columns, is wrapped at 120, and still parses.
  `.github/scripts/shell-lint.sh` clean; `mandoc -Tlint -W warning`: only the `.TH` date warning every page here
  carries. Until lane 61 lands, the five personal scripts that define their own `on_error_or_interrupt` keep it,
  theirs overriding the helper's, now under `errtrace`.

### Phase 3: The tests and the `Makefile`

- [ ] Requirement 5: `tests/Test-DeclareBashScript`, failing against the copied handler and passing against the
  helper's.
- [ ] Requirement 6: the `Makefile`'s `test` and `check`, and CI calling `make check`.

### Phase 4: The base's script

- [ ] Requirement 4.

### Phase 5: Verification

- [ ] `make check` clean on this Mac, and CI green on the pull request.
- [ ] `mandoc -Tlint -W warning` on `Declare-BashScript.1`.
- [ ] The deployed test, as ruled 2026-10-08 for Start-Claude ("checkout the branch in writ's clone for the test"):
  writ's base layer is `/Users/david-noble/Workspace/NobleFactor/noblefactor-ops`, so the owner checks this branch
  out there and runs `writ deploy`; the tests run against `~/.local/bin/Declare-BashScript`; the owner returns the
  clone to `develop` and deploys again.
- [ ] `--help` under the new helper for every consumer whose code before its option loop is only the guard, the
  `source` line and `require_*`, so nothing else runs.
- [ ] Every `$(...)` holding more than one command in the 70 scripts that source the helper, read for one that
  relies on running past a failure, which `inherit_errexit` would stop (question 4); each found is listed here with
  its fix, which lands before this does.

### Phase 6: Acceptance and closure

- [ ] The pull request, the merge, and `git close-branch`.
- [ ] Lanes 61 and 62 then carry the handler into personal's fourteen scripts and devlore-cli's three; the owner's
  acceptance is met when both have merged.

## Files

| File | Action | Purpose |
| --- | --- | --- |
| `Home/common/.local/bin/Declare-BashScript` | Modify | Requirement 1 |
| `Home/common/.local/share/man/man1/Declare-BashScript.1` | Modify | Requirement 3 |
| `scripts/setup-ground-zero.sh` | Modify | Requirement 4 |
| `tests/Test-DeclareBashScript` | Create | Requirement 5 |
| `Makefile` | Create | Requirement 6 |
| `.github/workflows/ci.yaml` | Modify | Requirement 6: CI calls `make check` |
| `docs/plans/chore/268-declare-bashscript-traps.md` | Create | This plan |

## Out of Scope

- Personal's fourteen scripts: David-Noble-at-work/personal#266, lane 61.
- devlore-cli's three: NobleFactor/devlore-cli#1037, lane 62.
- Whether the 57 scripts with no trap should trap: #267's guide rules it.
- Which bash files source the helper, beyond these five: #287.
- The man page's other omissions: #286.

## Open Questions

1. **The code a script writes. Ruled 2026-10-08, offered three `trap` lines naming the helper's handler (1) or one
   call to a helper function that sets them (2), and shown a script using `Set-Traps`: "2".** A script calls
   `Set-Traps`, with its cleanup function when it has one.
2. **How a signal ends the script. Ruled 2026-10-08, offered exiting with 128 plus the signal's number after the
   report (1), re-raising the signal (2), or running the cleanup in the handler and then re-raising (3): "1".** The
   EXIT trap's cleanup always runs, and the caller sees 130 or 143, not a death by signal.
3. **Which signals. Ruled 2026-10-08, offered INT, TERM and HUP (1) or INT and TERM, as today (2): "1".** First
   offered on the claim that an untrapped HUP ends a script without its cleanup; tested when the owner asked what HUP
   is, that was false: bash 5.3.15 runs the EXIT trap on an untrapped SIGHUP and exits 129. Asked again with that
   corrected, HUP being only whether a hangup gets the helper's report: "1".
4. **`inherit_errexit` for every consumer. Ruled 2026-10-08, offered on (1) or off (2): "1".** Measured first: it
   matters only for a `$(...)` holding several commands when one before the last fails. `x="$(false)"` stops either
   way; `x="$(false; echo hi)"` runs on today with `x=hi` and stops with it on; `echo "$(false)"` and
   `local x="$(false)"` run on either way. Before this lands, every `$(...)` holding more than one command in the 70
   scripts is read, and any that relies on running past a failure is listed in phase 5.
5. **Bash older than the helper needs. Ruled 2026-10-08, offered the helper refusing bash older than 4.4 with
   `EX_CONFIG`, the installers excepted (1), or setting `inherit_errexit` only where bash has it (2), after finding
   that `inherit_errexit` needs bash 4.4 and macOS's `/bin/bash` is 3.2.57: "We require a version of bash that tracks
   with what's current on Linux."** Measured the same day: Danoble-MBP-A runs MacPorts' 5.3.15 from a session or a
   login shell and Apple's 3.2.57 under launchd's default PATH; danoble-ud24-1, Ubuntu 26.04.1 LTS, runs 5.3.9. Then
   the floor, the owner: "I guess we can safely require 5.3 or higher". Measured after: on danoble-wd11-3, Git for
   Windows 2.55.0.windows.3 runs the `git-*` commands under bash 5.3.15. The installers, run as `curl ... | bash` on a
   fresh Mac, cannot meet the floor, and NobleFactor/devlore-cli#1037 and David-Noble-at-work/personal#266 settle how
   their embedded copies run.
6. **The tests. Ruled 2026-10-08, offered a committed script beside `.github/scripts/Test-Frontmatter.sh`, run by CI
   (1), or probes recorded in this plan's boxes, as #214 did (2): "Let's commit tests. They should be added to
   makefile. I don't want to stuff them into .github. They tend to go unnoticed there."** This repository has no
   `Makefile` (verified 2026-10-08); question 7 lays it out.
7. **The `Makefile`. Ruled 2026-10-08, offered a root `Makefile` modeled on devlore-cli's, whose `make test` runs the
   tests and whose `make check` runs every gate CI runs and then `test`, CI calling `make check`, with the tests in a
   top-level `tests/` directory (1), or a root `Makefile` with `make test` alone, which CI adds beside its present
   steps (2): "1".** Requirement 6.
8. **Where the `Set-Traps` call goes. Ruled 2026-10-08, offered after the cleanup function it names and before the
   arguments are parsed (1), or at the head of `Main` (2): "1".** A usage error or an interrupt during argument
   handling is reported and cleaned up.
