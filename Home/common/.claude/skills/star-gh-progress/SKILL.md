---
name: star-gh-progress
title: "star gh progress"
description: Answer "What is the status of your scheduled work?" and ANY other question about issues, epics, features, threads or schedules by running `star gh issues report` — never by ad-hoc `gh issue list`, `gh search`, or prose assembled from `gh issue view`. Triggers on "status", "what's the status", "status on the current issue", "lane status", "what is next", "show the schedule", "where do we stand", "what is open", "what covers this", "which bugs", and any request to see a schedule, epic, feature or thread. Use `star gh issues audit` for classification faults.
---

# Answering a question about issues

Ruled 2026-09-07, [agent-rules](../../../../../docs/guides/agent-rules.md) rule 3: *"What exists, what
is open, what covers a symptom, what a feature holds: `star gh issues report` and `star gh issues
audit`, never an ad-hoc `gh issue list --search`."* The breakage entry ends: *"it is our tool and we
spent significant resources to write that command. USE IT."*

`gh issue view <n>` is for reading one body **after** the report has named it.

## Status

The owner, 2026-10-05: *"when i ask for status, you should use the skill you've got to determine what
your assignment is and then report progress"*, and *"when i ask for status, i get full status. if i ask
for status on the current issue, i get status on the lane you're working."*

1. **The assignment.** When no schedule is named, take it from `star-gh-assignment`: the schedule this
   session works, `<owner>/<repository>#<number>`, and hand it to `--schedule` as it is.
2. **Status** is that schedule's lane table exactly as the first invocation below prints it. Nothing
   more: no lookups, no prose. *"the star gh issue report command ought to produce exactly what you
   need. no more. no less."*
3. **Status on the current issue**, or lane status, is the lanes being worked: the rows whose Next says
   "in progress", and the rows whose Next names the same pull request, printed as the report prints
   them. Next names each one's worktree. The owner: *"the pr that you do is the record of that work."*
4. **What's next?** One sentence: the next lane, the issue it maps to, and its worktree.
5. **What's next, in detail**, only when asked: the lanes in order, the pull requests that close them
   and what each waits on, ending with the table as the report prints it. *"i sometimes need that kind
   of detail. note my use of the qualif[i]er: sometimes."*

## Where to run it

From `~/Workspace/NobleFactor/noblefactor-ops`, whose `star/config.yaml` names all three
repositories. Elsewhere, pass `--repo` or `--directory`.

## Verify star first, and do not clobber it

`star` is a tool we build, so rule 1 applies: `git fetch` in devlore-cli — comparing against an
unfetched local `develop` proves nothing — then compare the installed build with `origin/develop`.

**If the installed build is `-dirty`, or `git branch -r --contains <build>` does not place it on
`develop`, it belongs to another session's work in progress.** Use it read-only and say so. Do not
`make install`: that replaces `~/.local/bin/star` and the extensions under
`~/.local/share/devlore/star/`, which is someone's working environment. A read-only report is not
worth that trade, and the report reads GitHub rather than the local tree, so a build a few commits
back answers the same question.

## The invocations

```bash
star gh issues report --by schedule --schedule <owner>/<repository>#<number> --markdown -o value --silent
star gh issues report --by feature --epic <Name>
star gh issues report --by thread --thread <Name>
star gh issues report --epic <Name> --view table --state open --markdown -o value --silent
star gh issues audit                       # and --unthreaded
```

**Always `--markdown -o value --silent`.** Without it the command returns raw JSON rows — every
field, alphabetical, including internal keys. Reformatting those by hand is exactly the behavior
rule 3 forbids, so the rule gets obeyed in letter and broken in substance.

`--state` defaults to open by epic, and to all by feature, thread and schedule.

## Two things to check before believing the output

**The lane-table header.** The report reads a schedule's lane columns 3 and 4 by position and prints
them as **Next** and **Waits on**. It does not check the header, so a table with different columns
renders confident nonsense rather than an error. The canonical shape is:

```
| Lane | Item | Next | Waits on |
```

If a schedule's table differs, those two columns are not what they say they are.

**The cost.** Roughly four minutes against three repositories. A deliberate call, not a per-turn one.
Ask for what is needed — one schedule, one epic — rather than the whole board.
