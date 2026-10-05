---
title: "star-gh-assignment names a session's schedule; star-gh-progress, once star-gh-report, reports its issues"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/257
status: draft
created: 2026-10-05
updated: 2026-10-05
---

# Plan: the star skills ask what a session works and how it stands

## Summary

The base layer ships one star skill, `star-gh-report`. This plan makes it two, named for the two questions the owner
asks a session, in order. `star-gh-assignment` answers "What schedule are you working on?" with the schedule issue,
read from the session's metadata. `star-gh-progress`, today's `star-gh-report` renamed, answers "What is the status of
your scheduled work?" by reporting every issue linked to that schedule. Start-Claude passes `/star-gh-assignment` as
the first prompt of a resume whose name carries no schedule (David-Noble-at-work/personal#260), so lanes 48, 49 and
51 of NobleFactor/devlore-cli#916 wait on this one, lane 50.

## Issue 257

Chore, epic `Ops:Process` (#142), feature #151; lane 50 of NobleFactor/devlore-cli#916. The owner, 2026-10-05: "we
first ask, what are you working on? we then ask, what's the status?"; on the names, "I like row four. It tracks your
progress on an assignment. The expression of an assignment is a schedule issue. We track progress by looking at all
issues linked to that schedule."; "Add a chore to update the star skills. When that's working, we'll move on to
Start-Claude and Start-Claude.ps1"; and "It's a change to noblefactor-ops". On what a session works: "your assignment
is always a schedule. always. we work one or more lanes at a time. the pr that you do is the record of that work."

## Goals

1. `star-gh-assignment` answers "What schedule are you working on?" with the schedule issue number,
   `<owner>/<repository>#<number>`, and nothing else.
2. It answers from the session's metadata and never from the conversation. The owner: "On 5: agreed. You answer from
   the metadata, not the transcript."
3. `star-gh-progress` answers "What is the status of your scheduled work?" by reporting every issue linked to the
   schedule, and keeps the epic, feature and thread reports `star-gh-report` gives today. The owner: "On 4: yes."
4. Passed as the first prompt of a resume, `claude --resume <id> "/star-gh-assignment"`, the skill runs and answers.
   Nothing asks it with `--print`: "Nothing asks a session except at a resume" (David-Noble-at-work/personal#260).
5. writ deploys both skills, and nothing links `star-gh-report` any longer.

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| `star-gh-report` | ✅ | Answers questions about issues, epics, features, threads and schedules (#237) |
| A session's own schedule | ❌ | No skill: a session answers from whatever its context holds, memory included |
| `star-gh-progress` and `star-gh-assignment` | ❌ | Neither exists |

## Requirements

### Requirement 1: `star-gh-progress`

`Home/common/.claude/skills/star-gh-report` moves to `Home/common/.claude/skills/star-gh-progress`, and its `name` and
`title` follow: `star-gh-progress`, "star gh progress". Its description leads with the question it answers, "What is
the status of your scheduled work?", and keeps today's triggers ("what is next", "show the schedule", "where do we
stand") and its epic, feature and thread reports. Asked for status with no schedule named, it first finds the
assignment with `star-gh-assignment`, then reports that schedule's progress: the whole schedule, or, asked for status
on the current issue, the lanes being worked (open question 2). The owner: "when i ask for status, i get full status.
if i ask for status on the current issue, i get status on the lane you're working." The body is otherwise unchanged.

### Requirement 2: `star-gh-assignment`

A new skill, `Home/common/.claude/skills/star-gh-assignment/SKILL.md`, answers "What schedule are you working on?"
from the session's name and nothing else: the last `custom-title` record in its transcript,
`${CLAUDE_CONFIG_DIR:-~/.claude}/projects/*/${CLAUDE_CODE_SESSION_ID}.jsonl`, read with one `grep`. A name in the
new form begins with the key, `<owner>/<repository>#<number> | `. A name in the form of
David-Noble-at-work/personal#257 claims nothing: "New form only. We'll rename sessions as we complete the work." The
session's branch names nothing: work starts in the main clone, and the worktrees come later. The owner: "i don't
start in a worktree. i usually don't know anything about the worktree til i look at the pr or ask about what you're
doing in the moment."

It reports the schedule issue number, `<owner>/<repository>#<number>`, and nothing else. The owner:
"star-gh-assignment reports the schedule issue number. that is all." Nothing in the conversation is read: no prompts,
no replies, no compaction summaries, no memory.

### Requirement 3: a first prompt

Start-Claude passes the skill as the first prompt of a resume whose name carries no key:
`claude --resume <id> --name <name> "/star-gh-assignment"`. The skill runs there, and its reads (`grep`, `gh`,
`star`) need no permission prompt. Phase 2 finds how.

### Requirement 4: deployment

After the merge, `writ deploy` runs. `~/.claude/skills/star-gh-progress/SKILL.md` and
`~/.claude/skills/star-gh-assignment/SKILL.md` resolve into the base layer, and no link named `star-gh-report`
dangles (agent-rules rule 5).

### Requirement 5: a lane in progress names its worktree

When work on a lane starts, its Next on the schedule says "in progress" and names the worktree, and the record is
kept current as the work moves; `star-gh-progress` shows it with the lane. `docs/issue-standards.md` § Schedules
states the rule. Offered it, the owner: "great."

## Implementation Phases

### Phase 1: The plan

- [ ] This document, committed, reviewed with the owner and approved, its questions ruled by the owner.

### Phase 2: What a skill can see

- [ ] `CLAUDE_CODE_SESSION_ID`, read inside a session, checked to name that session's transcript.
- [ ] `claude --resume <id> "/star-gh-assignment"` checked to run the skill as the first prompt, and how its reads
  run without a permission prompt. The owner runs whatever starts an interactive session; the agent cannot.

### Phase 3: `star-gh-progress`

- [ ] Requirement 1.
- [ ] Requirement 5, in `docs/issue-standards.md` § Schedules.

### Phase 4: `star-gh-assignment`

- [ ] Requirement 2, its description triggered by "what are you working on" and "what schedule are you working".

### Phase 5: Verify, then merge

- [ ] The gates CI runs, the frontmatter gate on both skills' `title` among them.
- [ ] The PR script written, shown and handed over.

### Phase 6: Deploy

These boxes close after the merge, so the plan stays `active` until they do.

- [ ] Requirement 4: `writ deploy`, both skills resolve into the layer, and no `star-gh-report` link dangles.
- [ ] Live on this Mac: asked "what are you working on?", a session named for #916 answers
  `NobleFactor/devlore-cli#916`; asked "what's the status?", it prints #916's lanes.
- [ ] Live: `claude --resume <id> "/star-gh-assignment"` answers for a session whose name carries no key.

## Migration Path

`star-gh-report` is renamed, not kept beside its successor. The plan for #237 keeps the old name as written.

## Files to Create/Modify

| File | Action | Purpose |
| --- | --- | --- |
| `docs/plans/chore/257-star-gh-assignment-names-the.md` | Create | This plan |
| `Home/common/.claude/skills/star-gh-report/SKILL.md` | Move | To `star-gh-progress/SKILL.md`, Requirement 1 |
| `Home/common/.claude/skills/star-gh-assignment/SKILL.md` | Create | Requirement 2 |
| `docs/issue-standards.md` | Modify | Requirement 5 |

## Related Documents

- `docs/plans/chore/237-base-layer-ships-a-star-gh.md`: the skill this renames (#237)
- David-Noble-at-work/personal#260: Start-Claude's rulings that call `star-gh-assignment` (lanes 48 and 49)
- David-Noble-at-work/personal#261: the ssh-agent for a login over SSH or mosh (lane 51)
- NobleFactor/devlore-cli#916, lane 50
- `docs/guides/agent-rules.md`, rule 3: a question about issues is answered by the report

## Open Questions

1. **Status with no schedule named. Ruled 2026-10-05: "when i ask for status, you should use the skill you've got to
   determine what your assignment is and then report progress."** `star-gh-progress` takes the schedule from
   `star-gh-assignment`.
2. **The lanes being worked. Ruled 2026-10-05.** The rows on the schedule whose Next names the pull request in
   progress. The owner: "we might open a worktree and resolve several lanes before we do a pr"; "the pr that you do is
   the record of that work"; and, correcting a reading of the session's branch, "i don't start in a worktree."
