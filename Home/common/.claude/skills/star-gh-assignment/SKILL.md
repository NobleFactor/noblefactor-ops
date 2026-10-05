---
name: star-gh-assignment
title: "star gh assignment"
description: Answer "What schedule are you working on?" with the schedule issue number, `<owner>/<repository>#<number>`, read from this session's name and nothing else. Triggers on "what are you working on", "what schedule are you working", "what is your assignment", and on `/star-gh-assignment` as a session's first prompt. Never reads the conversation, memory or the branch.
---

# What schedule this session works

The owner, 2026-10-05: *"star-gh-assignment reports the schedule issue number. that is all."* And:
*"your assignment is always a schedule. always."*

## How

Read the last name record in this session's transcript:

```bash
grep --only-matching --extended-regexp '"type":"custom-title","customTitle":"([^"\\]|\\.)*"' \
    "${CLAUDE_CONFIG_DIR:-${HOME}/.claude}"/projects/*/"${CLAUDE_CODE_SESSION_ID}".jsonl | tail --lines=1
```

A session's name begins with its schedule, `<owner>/<repository>#<number> | `. Answer with that key alone,
for example `NobleFactor/devlore-cli#916`. A name that does not begin with a key has no schedule to report:
answer `none`.

## What it never reads

The conversation (your prompts, the replies, compaction summaries), memory, and the session's branch. The
owner: *"You answer from the metadata, not the transcript."* A session starts in a repository clone on its
default branch, and its worktrees come later, so the branch names no schedule.
