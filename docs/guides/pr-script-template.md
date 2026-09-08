---
title: PR Script Template
description: Standard bash script structure for Claude Code to create, verify, merge, and clean up pull requests
type: Process
audience: Engineers, Claude Code
status: Approved
created: 2026-03-16
updated: 2026-09-07
---

# PR Script Template

Standard structure for the bash PR script that Claude Code generates to
`~/Workspace/NobleFactor/go`. Each script is self-contained: stage, commit, push,
create PR, verify CI, verify mergeability, squash merge, clean up.

---

## The pre-flight command is discovered, not remembered

The example below shows `.github/scripts/shell-lint.sh`, which is what the `personal` repository's
workflow invokes. **Do not copy it.** Read `.github/workflows/*` in the repository you are working
in and use what it actually runs:

| Repository | CI entry point |
| --- | --- |
| `personal` | `.github/scripts/shell-lint.sh`, with `DECLARE_BASHSCRIPT_DIR` naming a directory that holds `Declare-BashScript` (a deployed `~/.local/bin` locally; CI checks out `noblefactor-ops`) |
| `devlore-cli` | `make vet-all`, `make lint-all`, `./build/star lint go ./...` |
| `noblefactor-ops` | `./.github/scripts/Test-Frontmatter.sh`, `codespell`, `./.github/scripts/shell-lint.sh`, `buildifier -mode=check -lint=warn -warnings=-function-docstring-args,-function-docstring-return $(git ls-files '*.star')` |

Where a repository routes through a build tool, call the target rather than the underlying binary:
the target is what CI runs and it carries the flags. Where CI uses a GitHub Action rather than a
command — `codespell-project/actions-codespell`, say — run the CLI it wraps with the same
arguments the workflow passes it.

Read the workflow each time rather than trusting this table. It went stale during the change that
introduced it: `noblefactor-ops` dropped its Go trees and re-based its gate on documents while this
edit was in the working tree, turning a `go build` pre-flight into a command with nothing to
build.

A command from a tool this organization builds — `writ`, `star` — is verified against develop and
installed before it appears in a script or a message: [agent-rules.md](agent-rules.md), rule 1.

This step exists because a hand-rolled subset of the gate will differ from it eventually. On
2026-08-30 a `personal` PR pre-flighted with `shellcheck` alone and failed CI on `shfmt`, after the
issue, branch, commit and PR already existed. The remedy is not a longer checklist; it is to stop
maintaining a second copy of the gate.

---

## Template

```bash
#!/usr/bin/env bash
set -euo pipefail

cd ~/Workspace/NobleFactor/<repo>

# Stage specific files (never git add -A or git add .)
git add \
  path/to/file1 \
  path/to/file2

# Nothing left behind: a tracked file still modified after staging is a change this PR silently
# omits. gh prints "Warning: N uncommitted change" and carries on; the script must not.
git diff --quiet || {
  echo "unstaged changes remain; stage them by name or revert them:"
  git status --short
  exit 1
}

# --- Pre-flight: the repository's own gate, not an approximation of it ---
#
# Run what CI runs. Passing here must mean passing there, which is only true if it is literally
# the same command. Placed before the commit so a failure costs nothing -- no branch, no issue,
# no PR to recover.
#
# set -euo pipefail aborts on a non-zero exit. Do not wrap this in an if.
.github/scripts/shell-lint.sh

# Commit
git commit -m "$(cat <<'EOF'
<type>(<scope>): <summary>

<body>
EOF
)"

# Push branch
git push -u origin <branch>

# Create PR
gh pr create \
  --title "<type>(<scope>): <summary>" \
  --body "$(cat <<'EOF'
## Summary

- **Change 1** — description
- **Change 2** — description

## Test plan

- [x] Verified item
- [ ] TODO item

## Time

_Filled by the merge step below._
EOF
)"

# --- CI gate ---
sleep 5
pr_number=$(gh pr view --json number --jq '.number')

if ! gh pr checks "${pr_number}" --watch; then
    check_output=$(gh pr checks "${pr_number}" 2>&1 || true)
    if [[ "${check_output}" == *"no checks reported"* ]]; then
        echo "No CI checks configured, proceeding."
    else
        echo "CI checks failed:"
        echo "${check_output}"
        exit 1
    fi
fi

# --- Merge gate ---
merge_state=$(gh pr view "${pr_number}" --json mergeStateStatus --jq '.mergeStateStatus')
case "${merge_state}" in
    CLEAN|HAS_HOOKS|UNSTABLE|BLOCKED)
        ;; # OK to merge (--admin bypasses BLOCKED)
    DIRTY|BEHIND)
        echo "PR is not mergeable (state: ${merge_state})."
        if [[ "${merge_state}" == "DIRTY" ]]; then
            echo "Merge conflicts detected — resolve before merging."
        fi
        exit 1
        ;;
    *)
        echo "Unknown merge state: ${merge_state} — check PR manually."
        exit 1
        ;;
esac

# Squash merge -- and nothing else on this line.
#
# Always --squash. Never --delete-branch: it deletes the LOCAL branch first, which fails when a
# linked worktree holds the branch (the normal case under git-open-branch), gh exits non-zero, and
# set -e ends the script here -- after the merge has already landed, before any cleanup has run.
# Deletion belongs to git close-branch below, which deletes the remote first so a failure leaves
# both halves intact.
gh pr merge "${pr_number}" --squash --admin

# --- The span: every pull request records its time (rule 11) ---
#
# Three markers from GitHub's own timestamps, elapsed hours to one decimal, appended to the PR body and
# verified. AFTER the merge, not before: mergedAt does not exist until the merge lands. Carried verbatim
# here until `star gh pr span` exists (noblefactor-ops#188); then this block is one call to it.
span=$(gh pr view "${pr_number}" --json createdAt,mergedAt,commits --jq '
  (.commits[0].committedDate) as $opened | .createdAt as $submitted | .mergedAt as $merged |
  def hrs(a; b): (((b | fromdate) - (a | fromdate)) / 3600 * 10 | round) / 10;
  "## Time\n\n| Marker | UTC | Elapsed |\n| --- | --- | --- |\n" +
  "| opened (first commit) | \($opened) | — |\n" +
  "| submitted (PR opened) | \($submitted) | \(hrs($opened; $submitted)) h after opened |\n" +
  "| merged | \($merged) | \(hrs($submitted; $merged)) h after submitted; \(hrs($opened; $merged)) h in all |\n" +
  "\nComputed by the algorithm of noblefactor-ops#188 at merge."')
body=$(gh pr view "${pr_number}" --json body --jq .body | sed '/^## Time$/,$d')
printf '%s\n%s\n' "${body}" "${span}" | gh pr edit "${pr_number}" --body-file -
gh pr view "${pr_number}" --json body --jq .body | grep -q '^## Time' || {
    echo "the PR body carries no ## Time section"
    exit 1
}
printf '%s\n' "${span}"

# Clean up (works from both worktrees and regular branches)
git close-branch

# close-branch removed the worktree this script was standing in. Anything that runs after it must
# first step back to the main clone, or it runs from a deleted directory and exits 128.
cd ~/Workspace/NobleFactor/<repo>
git status --short

# --- The standing end-of-PR report (ruled 2026-09-04) ---
#
# Every pull request ends with the report, table form, all states, scoped to what the pull request
# served: --epic <Name>, --by feature --epic <Name>, --by thread --thread <Name>, or --by schedule.
# It is how the state the merge just changed is read back. --markdown -o value because a string
# result renders quoted under the default json (devlore-cli#826); --silent, not 2>/dev/null.
star gh issues report --epic <Name> --view table --state all --markdown -o value --silent
```

---

## Rules

1. **Stage by name.** Never `git add -A` or `git add .`.
2. **No Generated-by footer.** No `Co-Authored-By` lines in commit messages.
3. **HEREDOC for messages.** Both commit messages and PR bodies use `cat <<'EOF'` for safe formatting.
4. **CI gate is tolerant.** Repos with no required checks proceed; real failures abort.
5. **Merge gate before merge.** Check `mergeStateStatus` to catch conflicts, blocks, or staleness before attempting `gh pr merge`.
6. **`git close-branch` for cleanup.** Handles both worktree and regular-branch scenarios.
   It also **removes the worktree the script is standing in**, so nothing after it may assume `cwd`
   survives: `cd` to the main clone first. Observed as a spurious exit 128 after a successful merge
   on devlore-cli#811.
7. **One PR at a time.** Merge and clean up before starting the next branch.
8. **Always `--squash`, never `--delete-branch`.** The merge command carries `--squash` and no other
   deletion flag. `--delete-branch` deletes the local branch first, which fails when a linked
   worktree holds it, and under `set -e` that aborts the script after the merge and before cleanup.
   `git close-branch` deletes in the order that survives a failure: remote, then worktree, then local.
9. **Every pull request ends with the epic report.** Table form, all states, scoped to the epic,
   feature or thread the pull request served. Ruled 2026-09-04. The script prints it last, after
   cleanup, so the reader sees the state the merge produced.

10. **Nothing left behind.** After `git add` by name, `git diff --quiet` must pass. A modified tracked
    file the script did not stage is a change the PR silently omits; #161 merged without the rule
    above for exactly this reason.

11. **Every pull request records its time.** A `## Time` section in the pull request's own body, written by
    the merge step from GitHub's own timestamps and verified there: `opened` (the branch's first commit),
    `submitted` (the pull request's creation), `merged`, and the elapsed hours between them to one decimal.
    Ruled 2026-09-08. One algorithm for every pull request -- a number computed differently each time is
    noise, and the value is in the series. It measures how long the work was **open**, review gaps and nights
    included; it is not effort and must not be read as one. Where a coding agent estimates active time from
    its own session transcript, that estimate goes in the agent's end-of-PR message, labeled an estimate, and
    never into the pull request as fact.