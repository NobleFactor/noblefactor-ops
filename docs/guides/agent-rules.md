---
title: Agent Rules
description: Rules whose only audience is a coding agent — each written down the day an agent broke it
type: Process
audience: Claude Code, Codex
status: Approved
created: 2026-09-07
updated: 2026-09-07
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
`Declare-BashScript` among them, while the agent kept handing over the deploy command. "I need a
working system, not your bullshit."

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

---

A rule is added here the day it is broken, with the breakage. A rule an engineer would need belongs
in the engineers' documents, not here.
