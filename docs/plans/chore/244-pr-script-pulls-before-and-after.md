---
title: "The PR script pulls before and after the merge, and writes the Time section through the REST API"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/244
status: complete
created: 2026-09-24
updated: 2026-09-24
---

# Plan: the PR script pulls before and after the merge

## Issue 244

Every PR script written on 2026-09-24 — personal#221, #224, #226 and noblefactor-ops#242 — had to add
three things the template lacks: a pull onto the branch before the push, a pull of the main clone
after `git close-branch`, and the Time section written through the REST API because `gh pr edit`
dies on gh 2.46.0. The template is the one copy of the gate; a step every script adds by hand is a
step the template owes.

## Goals

1. A script generated from the template leaves the branch current with its target before CI judges
   it, and leaves the main clone current with the target after the merge.
2. The Time section lands on every gh version in use.
3. The template says nothing false about `git close-branch`.

## Requirements

### Requirement 1: Pull before the merge

In the bash template, between the commit and the push:

```bash
# Pull before the merge: bring the branch current with the target by a merge commit (never a rebase),
# so CI and the merge gate judge it against the target it will land on.
git pull --no-rebase --no-edit origin <target>
```

In the PowerShell form, the same before `git push`, followed by `Assert-NativeSuccess 'git pull'`.

### Requirement 2: Pull after the merge

In the bash template, after the `cd` that follows `git close-branch`:

```bash
# Pull after the merge: git close-branch switches and pulls the main worktree only when that worktree
# holds the branch being closed (#209). A branch from git open-branch lives in a linked worktree, so
# the main clone -- the layer writ deploys from -- is left behind without this.
git pull --ff-only origin <target>
git log --oneline -1
git status --short
```

In the PowerShell form, after `Set-Location $mainClone` and `git close-branch`: `git pull --ff-only
origin <target>` with its assert. Its rule 4, "No `git pull` after it: `git close-branch` ends by
fast-forwarding the main worktree", becomes the opposite: the pull is the script's, for the reason above.

### Requirement 3: The Time section through the REST API

The bash template's

```bash
printf '%s\n%s\n' "${body}" "${span}" | gh pr edit "${pr_number}" --body-file -
```

becomes

```bash
# Through the REST API, not `gh pr edit`: on gh 2.46.0 that command dies on GitHub's Projects (classic)
# deprecation (repository.pullRequest.projectCards) -- personal#221's run stopped after its merge with
# no Time section and no cleanup. The Windows form has always written the body this way.
body_file=$(mktemp); trap 'rm -f "${body_file}"' EXIT
printf '%s\n%s\n' "${body}" "${span}" > "${body_file}"
gh api -X PATCH "repos/<owner>/<repo>/pulls/${pr_number}" -F "body=@${body_file}" --jq .number >/dev/null
```

The verification `grep -q '^## Time'` that follows stays.

### Requirement 4: The rules list

Rule 6 ("`git close-branch` for cleanup") loses nothing but gains the pull. A new rule 12: **Pull before
and after the merge.** Ruled 2026-09-24. Before: the branch takes the target by merge commit, so what
CI passes is what merges. After: the main clone takes the squash commit, because `git close-branch`
does not.

### Requirement 5: The document's own metadata

`updated:` becomes 2026-09-24.

## Implementation Phases

### Phase 1: Commit the plan

- [x] This plan, before the change

### Phase 2: The template

- [x] Requirement 1, bash and PowerShell
- [x] Requirement 2, bash and PowerShell, and the Windows rule 4 corrected
- [x] Requirement 3
- [x] Requirement 4
- [x] Requirement 5

### Phase 3: Verify, then merge

- [x] The bash template block, extracted with its placeholders substituted: `bash -n` clean; shellcheck
      0.11.0 reports one pre-existing info, SC2016 on the jq program's single quotes; shfmt v3.14.1's
      diff is the block's pre-existing two-space indentation. Neither is this change's, and the block
      is prose, not a tracked script
- [x] Frontmatter (50 checked, 0 errors) and codespell (CI's arguments) pass on DANOBLE-UD24-1;
      PSScriptAnalyzer is not installed here, the change touches no `.ps1`, and CI runs that gate
- [x] PR script written, shown, and handed over — itself carrying all three changes, as today's did

## Out of Scope

- **Whether `git close-branch` should fast-forward the main clone itself.** #209 made the switch
  conditional on purpose; the script's pull is the remedy that needs no change to the command. If the
  command should do it, that is its own chore.
- **`git-submit-branch`** (#145), which would replace the template with a command. This plan corrects
  the template that exists.
