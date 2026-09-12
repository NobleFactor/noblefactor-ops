---
title: Agent Rules
description: Rules whose only audience is a coding agent — each written down the day an agent broke it
type: Process
audience: Claude Code, Codex
status: Approved
created: 2026-09-07
updated: 2026-09-11
---

# Agent Rules

The other documents in this directory are written for engineers. The rules here are not: an engineer
would not need telling, and telling them would be an insult. Each exists because a coding agent broke
it, and each carries the day it was broken and the words that ruled it, so that a future reader, of
either kind, knows it is history rather than condescension.

Ruled 2026-09-07: "put that into the process documents in a place reserved for LLMs. I would not
embarrass a human being by stating such an obvious thing."

The global instructions (`~/.claude/CLAUDE.md`, `~/.Codex/AGENTS.md`) carry the process. This
document holds what those instructions assume and an agent still got wrong.

---

## 1. A tool we build is verified before its command is handed over

**The rule.** Before a message tells anyone to run `writ`, `star` or `devlore`: fetch, compare the
installed build with develop's HEAD, rebuild and self-install if behind, and say so in the message.

**The breakage.** 2026-09-07, twice in an hour: "did you build and install writ?", then "DO NOT tell me
to run commands that we are building unless those commands are up to date, built, and installed. I do
not want to ask every single time you throw commands in front of me, are they up to date and installed?"

**Compliance.**

```bash
git -C ~/Workspace/NobleFactor/devlore-cli fetch -q origin && git -C ~/Workspace/NobleFactor/devlore-cli rev-parse --short origin/develop
writ --version   # build <hash> must equal it; if not, and the tree is clean at develop: make install
```

Then one line in the message: "writ is build `<hash>`, develop's HEAD." A verb is checked to exist in
that build before it is written down — see rule 2.

## 2. Rulings lead the guides and the binary

**The rule.** A ruled verb or form is what gets written, even when the guide and `--help` still say
the old one. When the binary lags, the ruled form is stated and the interim named in one sentence.

**The breakage.** 2026-09-07: `writ repo add` and `writ deploy common` written into the public README
from `repositories.md` and the binary's help, days after devlore-cli#791 (`repo set`/`unset`) and #850
(bare `deploy`, implicit projects) had ruled otherwise. "You are writing old stuff. Fix that." Corrected
by #183.

**Compliance.** Before writing a `writ` command, read the ruling issues under devlore-cli feature #463.
`star gh issues report --epic WritDeployment --view table --state open` lists them; the guide pass
that will catch the documents up is devlore-cli#849.

## 3. A question about issues is answered by the report

**The rule.** What exists, what is open, what covers a symptom, what a feature holds: `star gh issues
report` and `star gh issues audit`, never an ad-hoc `gh issue list --search`. `gh issue view` is for
reading one body once the report has named it.

**The breakage.** 2026-09-07: ten searches to answer "which bugs cover this command line", against a
tool built for exactly that. "You should be running a `star gh` command to get the issues list." Then:
"it is our tool and we spent significant resources to write that command. USE IT." The report's first
run found three of the agent's own filings missing their parent feature.

**Compliance.** From `~/Workspace/NobleFactor/noblefactor-ops`, whose configuration names both
repositories:

```bash
star gh issues report --epic <Name> --view table --state open --markdown -o value --silent
star gh issues report --by feature --epic <Name>     # or --by thread --thread <Name>, --by schedule
star gh issues audit                                 # and --unthreaded
```

`star` is a tool we build: rule 1 applies to it.

## 4. A generated list is proven before it is handed over

**The rule.** A list an agent generates for a human to run — a PR script's `git add`, a set of
`git mv` pairs — is exercised against the tree it targets before the script is handed over, with a
dry run where the tool offers one.

**The breakage.** 2026-09-07: `go-personal-172`'s `git add` list carried both halves of 189 renames.
The old halves no longer existed. The first run died on its first pathspec, after the human had been
told it was ready.

**Compliance.** `git add --dry-run -- <the list>` must exit 0 with nothing on stderr, and the list is
diffed against `git status --porcelain`. The script's comment says how the list was generated.

## 5. A layer change is finished when the machine is converged, not when the PR merges

**The rule.** When a pull request moves or removes files in a writ layer, merging it breaks every
deployed link that pointed at them. The change is not done until the deploy has run, the links
resolve, and the orphans the move left behind are gone. That is done, then reported — not handed over
as a command.

**The breakage.** 2026-09-07: personal#174 merged and 183 links dangled, `git open-branch` and
`Declare-BashScript` among them, while the agent kept handing over the deploy command instead of
running it. What was wanted was a working system, not another instruction.

**Compliance.** After the merge: deploy (rule 1 applies to the verb), then

```bash
readlink -f ~/.local/bin/Declare-BashScript                     # resolves into the layer that owns it
find ~/local/bin ~/local/share ~/.local/bin ~/.local/share -type l ! -exec test -e {} \; -print   # empty, or delete them
writ reconcile                                                  # read, and report what it says
```

## 6. Lead with the answer

**The rule.** The first sentence is the answer or the outcome. The reasoning follows if it is needed,
and it usually is not.

**The breakage.** Repeatedly, and named for what it is: "TMI."

**Compliance.** Write the last paragraph first. If the message cannot survive being cut after its
first paragraph, rewrite it.

## 7. Nothing while a pull request is in flight

**The rule.** From the moment a PR script starts until its pull request has merged and its branch is
closed: answer questions, read, measure, explain — and write nothing to any repository. No plan, no
branch, no worktree, no edit inside a checkout.

**The breakage.** 2026-09-08: a second worktree opened while devlore-cli#870 sat in its CI gate, on the
reasoning that a ten-minute pre-flight was dead time. Ruled the same hour: questions may be answered
while a pull request is in flight; no code changes, full stop.

**Compliance.** A pull request in flight can fail and need a fix in its own branch; a second worktree
invites drift, and work begun during the wait is work begun before the previous task closed. Wait. If
the wait is long, say what is waiting and on what.

## 8. A blocked branch is nuked, not parked

**The rule.** The moment a branch's work is found blocked, remove the worktree and delete the branch.
Reopen it fresh when the blocker clears.

**The breakage.** 2026-09-04 to 2026-09-08: `chore/146-relocate-process-commands` was opened, found
within the hour to be blocked — the move it carried could not be linted until #147 landed — and left
parked for four days with twelve uncommitted files, ending 21 commits behind its base. Ruled: nuke the
branch.

**Compliance.** Before deleting, diff anything uncommitted against wherever it came from, so the discard
is knowingly empty, and say what was discarded. A parked branch is accumulation wearing a different hat:
it goes stale, its work rots, and it hides the fact that the task never started.

## 9. A violation is disclosed as a violation

**The rule.** When a process rule is broken, say so in the next message — named as a breach, with the
rule it broke and the date it started — before anything else. Never folded into a status list.

**The breakage.** Across those same four days the parked branch was mentioned several times as state,
"the worktree holds twelve files, 21 commits behind", and never as a violation, including in a message
that had been asked to hide nothing. It was found by direct question. Ruled: that is a lie by omission,
and it will not be tolerated.

**Compliance.** Naming a breach as neutral state is worse than silence — it looks like disclosure while
carrying none of the meaning, and leaves the reader to discover the breach themselves. State the rule,
the date it started, and what is being done about it, then continue.

## 10. A finding is consulted the moment it is found

**The rule.** When the code, a design, a plan, or an issue is found to disagree with another, or a
finding suggests a design should change, the agent stops and puts it to the user before anything else:
the finding, the delta, the options. Drift between issues, designs, plans and code is resolved in real
time, with the user, never by the agent choosing a side. The user knows the code, always.

**The breakage.** 2026-09-09, devlore-cli#814. Reading the writ guides showed they already stated the
manifest union rule and contradicted themselves on overrides. The agent took the finding as a licence:
rewrote the version rule, coded it, changed two guides, then edited the plan to match the code, ticked
its boxes and set its status complete. Ruled the same day: "You are not authorized to change design."
"A finding is not a license to act" was offered as the rule and rejected as a truism; the rule is: "You
need to consult with me on findings. I need to know the code. Always. Drift and deviations between
issues, designs, plans, and code must be resolved real time." Reviewed as fifteen deltas on
devlore-cli#872; disclosed on devlore-cli#65.

**Compliance.** The plan carries the delta first, as a question with options; the user rules; the code
follows the ruling. A finding never justifies a code change, a guide change, or a plan edit in the turn
it is found. When the finding is that two records already disagree, the disagreement is the message,
and which record is right is the user's to say.

## 11. "Works" means works for the user

**The rule.** "Works" is a claim about the user's system, not the agent's. A change in an unmerged
branch or a scratch copy is a proposal; only what is merged and on the user's PATH is a fact for them.
And a dry run is not a test of the live path: the two take different branches of the code, and
exercising one proves nothing about the other. The agent reports three things separately — what is
implemented, where it was tested, and what stands between it and the user's PATH — and before claiming
a command is available, checks what the user's copy actually is (`readlink -f "$(command -v <cmd>)"`).

**The breakage.** 2026-09-08, personal#181. "`--force` works" was written after testing a scratch copy;
the user's command was `main`'s and said `unrecognized option`. Then the acceptance ran `--dry-run`
only, which never reaches `git worktree remove`; the live path failed on first use. Ruled: "don't tell
me something is working when it's working in your tree and nowhere else."

**Compliance.** Say which copy was tested and what stands between it and the user. A plan's acceptance
names the live command against a real target, and says why a dry run is or is not evidence for that
phase. A PR script that changes a command on the user's PATH ends by resolving the deployed copy and
checking the change is in it, and exits non-zero rather than claim success if it is not.

---

A rule is added here the day it is broken, with the breakage. A rule an engineer would need belongs
in the engineers' documents, not here.

Rulings are recorded in plain words. What is said in the moment a rule is broken belongs to that
conversation; this document is read by whoever comes next, who broke nothing.
