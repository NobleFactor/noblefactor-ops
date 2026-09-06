# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

"""The NobleFactor issue classification scheme, as Get-EpicReport's jq states it."""

# scheme.star
#
# Every definition here is a line-for-line port of a jq def in devlore-cli's scripts/Get-EpicReport,
# kept in the same order under the same names so the two can be read side by side until the script
# retires (devlore-cli#797). The scheme itself is docs/issue-standards.md.

KINDS = ["epic", "feature", "task", "bug", "chore"]

def short(repo):
    """The repository's name without its owner: noblefactor-ops for NobleFactor/noblefactor-ops."""
    return repo.split("/")[-1]

def key(it):
    """owner/repo#N -- the identity a **Feature:** marker names across repositories."""
    return it["repo"] + "#" + str(it["number"])

def names(it):
    return [label["name"] for label in it["labels"]]

def tagged(it, label):
    return label in names(it)

def kinds(it):
    return [n for n in names(it) if n in KINDS]

def epic_tags(it):
    return [n for n in names(it) if n.startswith("Epic:")]

def epic_tag(it):
    tags = epic_tags(it)
    return tags[0] if tags else ""

def mark(it):
    return "~~" if it["state"] == "CLOSED" else ""

def line(it):
    return mark(it) + it["title"] + mark(it) + " [" + it["ref"] + "](" + it["url"] + ")"

def is_bug(it):
    return tagged(it, "bug")

def _first_with_prefix(it, prefix):
    for n in names(it):
        if n.startswith(prefix):
            return n
    return ""

def severity(it):
    return _first_with_prefix(it, "Severity:")

def priority(it):
    return _first_with_prefix(it, "Priority:")

def severity_rank(it):
    s = severity(it)
    return {"Severity:Critical": 0, "Severity:High": 1, "Severity:Medium": 2, "Severity:Low": 3}.get(s, 4)

# A tier-three issue is a task, a bug, or a chore; they are peers under a feature. One with no parent
# feature falls through to "unfiled" like any other child.
def is_child(it):
    return tagged(it, "task") or is_bug(it) or tagged(it, "chore")

# A task's parent feature, stated in its body as **Feature:** #123 or **Feature:** owner/repo#123.
# The marker is the closing metadata line, so the LAST match wins: prose above it may mention another
# feature in the same form, and the script's narrower regex happened never to see those.
def parent_feature(it):
    """The key of the parent feature the body names -- owner/repo#N -- or None. A bare #N is the issue's own repository."""
    ms = regex.find_all_submatch(pattern = "Feature:\\*\\* (?:([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+))?#([0-9]+)", text = it.get("body", "") or "", count = -1)

    # find_all_submatch returns None on no match; an optional group that did not participate is "".
    if ms == None or len(ms) == 0:
        return None
    m = ms[-1]
    return (m[1] if m[1] else it["repo"]) + "#" + m[2]

# A bug awaiting triage has no feature yet, so it has no epic. That is the triage queue, not a fault.
def awaiting_triage(it):
    return is_bug(it) and len(epic_tags(it)) == 0

# A thread issue is a scenario, and a scenario has no owner: it carries no Epic: label and is placed
# by its own label family. The second exception (issue-standards.md, The two exceptions).
def is_thread_issue(it):
    return tagged(it, "feature") and it["title"].startswith("Thread:") and len([n for n in names(it) if n.startswith("Thread:")]) > 0

# A schedule issue is the declared order of execution -- a view of the work, not work -- and carries
# no Epic: label either. The three slices are peers (issue-standards.md, Three ways to slice the work).
def is_schedule_issue(it):
    return tagged(it, "chore") and it["title"].startswith("Schedule:")

def is_view_issue(it):
    """A thread or schedule issue: organizes work, is not work, carries no epic."""
    return is_thread_issue(it) or is_schedule_issue(it)

def faults(it):
    """The classification faults of an open issue: wrong count of kind labels, wrong count of epic labels."""
    if awaiting_triage(it):
        return []
    k = kinds(it)
    e = epic_tags(it)
    out = []
    if len(k) == 0:
        out.append("no kind label (epic/feature/task/bug/chore)")
    if len(k) > 1:
        out.append("multiple kind labels: " + ", ".join(k))
    if len(e) == 0 and not is_view_issue(it):
        out.append("no Epic:<Name> label")
    if len(e) > 1:
        out.append("multiple epics: " + ", ".join(e))
    return out

def kind_of(it):
    """The one kind an issue renders as, epic first; unclassified when it carries none."""
    if tagged(it, "epic"):
        return "epic"
    if tagged(it, "feature"):
        return "feature"
    if is_bug(it):
        return "bug"
    if tagged(it, "task"):
        return "task"
    if tagged(it, "chore"):
        return "chore"
    return "unclassified"

def cell(s):
    return s.replace("|", "\\|").replace("\n", " ")

def _strip(s, prefix):
    return s[len(prefix):] if s.startswith(prefix) else s

def comment(it, placement):
    """The Comment column: kind, placement, WIP, severity, priority, and any open fault."""
    parts = [kind_of(it), placement]
    if tagged(it, "WIP"):
        parts.append("WIP")
    sev = _strip(severity(it), "Severity:")
    if sev:
        parts.append("severity " + sev)
    pri = _strip(priority(it), "Priority:")
    if pri:
        parts.append("priority " + pri)
    if it["state"] == "OPEN":
        f = "; ".join(faults(it))
        if f:
            parts.append(f)
    return "; ".join([p for p in parts if p != ""])

def row(it, placement):
    return {
        "issue": it["number"],
        "repo": it["repo"],
        "ref": it["ref"],
        "url": it["url"],
        "title": cell(it["title"]),
        "kind": kind_of(it),
        "state": it["state"],
        "done": it["state"] == "CLOSED",
        "placement": placement,
        "severity": _strip(severity(it), "Severity:"),
        "priority": _strip(priority(it), "Priority:"),
        "wip": tagged(it, "WIP"),
        "faults": faults(it) if it["state"] == "OPEN" else [],
        "comment": comment(it, placement),
    }

def task_row(it):
    done = "✅" if it["state"] == "CLOSED" else ""
    kind = "bug" if is_bug(it) else "task"
    return "| " + done + " | [" + it["ref"] + "](" + it["url"] + ") | " + mark(it) + cell(it["title"]) + mark(it) + " | " + kind + " | " + _strip(severity(it), "Severity:") + " | " + _strip(priority(it), "Priority:") + " |"

def by_number(items):
    return sorted(items, key = lambda i: (i["number"], i["repo"]))

# The rows of one epic, in tree order: the epic, then each feature and its children, then the unfiled.
def epic_rows(e, all):
    """The rows of one epic in tree order: the epic, each feature and its children, then the unfiled."""
    tag = epic_tag(e)
    features = by_number([i for i in all if tagged(i, "feature") and epic_tag(i) == tag and not tagged(i, "epic")])
    tasks = [i for i in all if is_child(i) and epic_tag(i) == tag]
    feature_keys = [key(f) for f in features]
    rows = [row(e, "")]
    for f in features:
        rows.append(row(f, ""))
        for t in by_number([t for t in tasks if parent_feature(t) == key(f)]):
            r = row(t, "feature " + f["ref"])
            r["feature"] = f["number"]
            r["feature_repo"] = f["repo"]
            rows.append(r)
    for t in by_number([t for t in tasks if parent_feature(t) == None or parent_feature(t) not in feature_keys]):
        rows.append(row(t, "unfiled, no parent feature"))
    if len(features) == 0 and len(tasks) == 0:
        rows[0]["comment"] = rows[0]["comment"] + "; awaiting plan and design"
        rows[0]["placement"] = "awaiting plan and design"
    for r in rows:
        r["epic"] = e["number"]
        r["axis"] = axis(epic_name(e))
    return rows

def table_lines(rows):
    out = ["| ID | Issue | Title | Status | Comment |", "|---|---|---|---|---|"]
    for i, r in enumerate(rows):
        out.append("| " + str(i + 1) + " | [" + r["ref"] + "](" + r["url"] + ") | " + r["title"] + " | " + ("✅" if r["done"] else "") + " | " + r["comment"] + " |")
    return out

# ── fetching ────────────────────────────────────────────────────────────────

def _origin_repo(directory):
    """owner/name from the origin remote of a working tree."""
    r = shell.exec(command = "git -C '" + directory + "' remote get-url origin")
    m = regex.find_submatch(pattern = "[:/]([^/:]+/[^/]+?)(?:\\.git)?\\s*$", text = r.stdout)
    if m == None or len(m) < 2:
        fail("cannot read owner/name from the origin of " + directory + ": " + r.stdout.strip())
    return m[1]

def _current_repo():
    """owner/name of the repository the command runs in, as gh resolves it."""
    r = shell.exec(command = "gh repo view --json nameWithOwner --jq .nameWithOwner")
    return r.stdout.strip()

def configured_repos():
    """gh.repositories from star/config.yaml, or an empty list."""
    cfg = config.get
    if hasattr(cfg, "gh") and hasattr(getattr(cfg, "gh"), "repositories"):
        return [str(x) for x in getattr(cfg, "gh").repositories]
    return []

def resolve_repos(repo_flag, directory):
    """The repositories to report on: --repo, else --directory's origin, else gh.repositories, else the current one."""
    if repo_flag:
        return [r.strip() for r in repo_flag.split(",") if r.strip()]
    if directory:
        return [_origin_repo(directory)]
    repos = configured_repos()
    return repos if repos else [_current_repo()]

def fetch_issues(state, limit, repos):
    """Every issue of each repository via gh issue list -- authoritative and immediate, unlike search -- tagged with its repository and display ref."""

    # shell.exec raises on a non-zero exit (devlore-cli#824), so a failed gh never reaches this code;
    # there is no exit_code to check.
    multi = len(repos) > 1
    issues = []
    for repo in repos:
        r = shell.exec(command = "gh issue list -R " + repo + " --state " + state + " --limit " + str(limit) + " --json number,title,state,labels,body,url")
        batch = json.decode(data = r.stdout)
        for it in batch:
            # encoding/json decodes numbers as floats; the scheme compares and prints issue numbers as ints.
            it["number"] = int(it["number"])
            it["repo"] = repo
            it["ref"] = (short(repo) + "#" if multi else "#") + str(it["number"])
        issues.extend(batch)
    return issues

def label_space(repos):
    """Every Epic: and Thread: label per repository with its open count -- one GraphQL request, filtered here and never with labels(query:), which is a relevance search."""
    parts = []
    for i, repo in enumerate(repos):
        owner, name = repo.split("/")
        parts.append("r" + str(i) + ": repository(owner: \"" + owner + "\", name: \"" + name + "\") { nameWithOwner labels(first: 100) { nodes { name issues(states: OPEN) { totalCount } } } }")
    r = shell.exec(command = "gh api graphql -f query='{ " + " ".join(parts) + " }'")
    data = json.decode(data = r.stdout)["data"]
    out = {}
    for k in data:
        node = data[k]
        out[node["nameWithOwner"]] = {label["name"]: int(label["issues"]["totalCount"]) for label in node["labels"]["nodes"] if label["name"].startswith("Epic:") or label["name"].startswith("Thread:")}
    return out

def exempt_numbers():
    # gh.exempt in star/config.yaml. Flat, not gh.issues.exempt: the extension spec accepts a nested:
    # block but the runtime does not build it into the config struct (noblefactor-ops#140, Phase 1).
    cfg = config.get
    if not hasattr(cfg, "gh") or not hasattr(getattr(cfg, "gh"), "exempt"):
        return []
    return [int(n) for n in getattr(getattr(cfg, "gh"), "exempt")]

# ── the audit ───────────────────────────────────────────────────────────────

def audit(all, exempt):
    """The classification audit: the faulty, the exempted, the open bugs, and those awaiting triage."""
    open_ = [i for i in all if i["state"] == "OPEN"]
    bad = by_number([i for i in open_ if i["number"] not in exempt and len(faults(i)) > 0])
    exempted = by_number([i for i in open_ if i["number"] in exempt])
    bugs = [i for i in open_ if is_bug(i)]
    untriaged = by_number([i for i in bugs if awaiting_triage(i)])
    return {"bad": bad, "exempted": exempted, "bugs": bugs, "untriaged": untriaged}

def audit_lines(a):
    """The audit section as the script prints it."""
    out = []
    if len(a["bad"]) == 0:
        out.append("All open issues carry exactly one kind label and exactly one epic.")
    else:
        out.append("**" + str(len(a["bad"])) + " issue(s) are not properly classified.** An issue the scheme cannot place is invisible to the tree below.\n")
        for i in a["bad"]:
            out.append("- " + line(i) + " — " + "; ".join(faults(i)))
    if len(a["exempted"]) > 0:
        out.append("\nDeliberately outside the scheme, by decision:\n")
        for i in a["exempted"]:
            out.append("- " + line(i))
    if len(a["untriaged"]) > 0:
        out.append("\n**" + str(len(a["untriaged"])) + " bug(s) awaiting triage** — no feature, so no epic. Not a fault; this is the queue.\n")
        for i in a["untriaged"]:
            out.append("- " + line(i))
    return out

# ── threads ─────────────────────────────────────────────────────────────────
#
# A thread is a narrative -- a use case or scenario -- carrying cross-cutting feature development
# (docs/issue-standards.md, Threads). Membership is the Thread:<Name> label; order is the beat table
# in the thread issue's body; the two must agree, and disagreement either way is a fault.

def thread_names(space):
    """Every thread across the configured repositories, from the label space, sorted."""
    names = {}
    for repo in space:
        for label in space[repo]:
            if label.startswith("Thread:"):
                names[label[len("Thread:"):]] = True
    return sorted(names.keys())

def thread_issue(all, name):
    """The thread's own issue: kind feature, carrying its label, titled 'Thread:'. None when absent."""
    candidates = by_number([i for i in all if tagged(i, "feature") and tagged(i, "Thread:" + name) and i["title"].startswith("Thread:")])
    return candidates[0] if candidates else None

def beat_table(thread):
    """(beat, owner/repo#N) in document order, from rows of the thread issue's body whose first cell is a beat number."""
    beats = []
    for raw in (thread.get("body", "") or "").split("\n"):
        cells = [c.strip() for c in raw.strip().strip("|").split("|")]
        if len(cells) < 2 or not regex.match(pattern = "^[0-9]+$", text = cells[0]):
            continue
        refs = regex.find_all_submatch(pattern = "(?:([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+))?#([0-9]+)", text = raw, count = -1)
        if refs == None or len(refs) == 0:
            continue
        m = refs[0]
        beats.append((int(cells[0]), (m[1] if m[1] else thread["repo"]) + "#" + m[2]))
    return beats

def thread_rows(name, all):
    """The thread issue, then its members in beat order, then labelled members the table omits; with the agreement faults."""
    thread = thread_issue(all, name)
    members = [i for i in all if tagged(i, "Thread:" + name) and (thread == None or key(i) != key(thread))]
    by_key = {key(i): i for i in members}
    beats = beat_table(thread) if thread else []
    listed = {k: b for b, k in beats}

    rows = []
    faults = []
    if thread == None:
        faults.append("no thread issue: no feature titled 'Thread:' carries Thread:" + name)
    else:
        r = row(thread, "")
        r["thread"] = name
        r["beat"] = ""
        rows.append(r)

    for b, k in beats:
        if k in by_key:
            r = row(by_key[k], "beat " + str(b))
            r["thread"] = name
            r["beat"] = b
            r["epic_name"] = _strip(epic_tag(by_key[k]), "Epic:")
            rows.append(r)
        else:
            faults.append("beat " + str(b) + " names " + k + ", which does not carry Thread:" + name)

    for i in by_number([i for i in members if key(i) not in listed]):
        r = row(i, "labelled, not in the beat table")
        r["thread"] = name
        r["beat"] = ""
        r["epic_name"] = _strip(epic_tag(i), "Epic:")
        rows.append(r)
        faults.append(i["ref"] + " carries Thread:" + name + " and is not in the beat table")

    return {"name": name, "thread": thread, "rows": rows, "faults": faults}

def thread_table_lines(rows):
    """The thread's status table: beat, issue, title, status, owning epic, comment."""
    out = ["| Beat | Issue | Title | Status | Epic | Comment |", "|---|---|---|---|---|---|"]
    for r in rows:
        out.append("| " + str(r.get("beat", "")) + " | [" + r["ref"] + "](" + r["url"] + ") | " + r["title"] + " | " + ("✅" if r["done"] else "") + " | " + r.get("epic_name", "") + " | " + r["comment"] + " |")
    return out

# ── the scheme's semantics: axes, and what an empty level means ────────────
#
# docs/issue-standards.md: the Ops: segment marks the tooling axis; an epic with no features is
# awaiting plan and design; a feature with no tasks is awaiting decomposition; a child with no
# feature is unfiled. The report says which, rather than rendering blank.

def axis(name):
    """tooling for a name carrying the Ops: segment, else product."""
    return "tooling" if name.startswith("Ops:") else "product"

def by_axis(items, name_of):
    """Product first, then tooling; stable within each."""
    return [i for i in items if axis(name_of(i)) == "product"] + [i for i in items if axis(name_of(i)) == "tooling"]

def epic_name(it):
    """The Name in an issue's Epic:<Name> label, or empty."""
    return _strip(epic_tag(it), "Epic:")

def children_of(f, tasks):
    """The tier-three issues that name feature f as their parent."""
    return by_number([t for t in tasks if parent_feature(t) == key(f)])

def feature_state(f, kids):
    """The named condition of a feature: closed; awaiting decomposition; children done; or n of m done."""
    if f["state"] == "CLOSED":
        return "closed"
    if f["title"].startswith("Thread:"):
        return "a thread; its members report under --by thread"
    if len(kids) == 0:
        return "awaiting decomposition"
    done = len([k for k in kids if k["state"] == "CLOSED"])
    if done == len(kids):
        return "children done"
    return str(done) + " of " + str(len(kids)) + " done"

def feature_rows(epics, all):
    """One row per feature across the given epics, with its counts and state; an epic with none contributes a row saying so."""
    rows = []
    for e in epics:
        tag = epic_tag(e)
        features = by_number([i for i in all if tagged(i, "feature") and epic_tag(i) == tag and not tagged(i, "epic")])
        tasks = [i for i in all if is_child(i) and epic_tag(i) == tag]
        if len(features) == 0:
            rows.append({
                "epic": e["number"],
                "epic_ref": e["ref"],
                "epic_name": epic_name(e),
                "axis": axis(epic_name(e)),
                "feature": None,
                "ref": "",
                "url": e["url"],
                "title": "",
                "done": 0,
                "open": 0,
                "state": "awaiting plan and design" if len(tasks) == 0 else "awaiting plan and design; " + str(len(tasks)) + " unfiled",
            })
            continue
        for f in features:
            kids = children_of(f, tasks)
            done = len([k for k in kids if k["state"] == "CLOSED"])
            rows.append({
                "epic": e["number"],
                "epic_ref": e["ref"],
                "epic_name": epic_name(e),
                "axis": axis(epic_name(e)),
                "feature": f["number"],
                "repo": f["repo"],
                "ref": f["ref"],
                "url": f["url"],
                "title": cell(f["title"]),
                "closed": f["state"] == "CLOSED",
                "done": done,
                "open": len(kids) - done,
                "state": feature_state(f, kids),
            })
    return rows

def feature_table_lines(rows):
    """The feature rollup: feature, done, open, state."""
    out = ["| Feature | Done | Open | State |", "|---|---|---|---|"]
    for r in rows:
        if r["feature"] == None:
            out.append("| _no features_ |  |  | " + r["state"] + " |")
        else:
            out.append("| [" + r["ref"] + "](" + r["url"] + ") " + ("~~" + r["title"] + "~~" if r["closed"] else r["title"]) + " | " + str(r["done"]) + " | " + str(r["open"]) + " | " + r["state"] + " |")
    return out

# ── the label set every participating repository carries ───────────────────
#
# docs/issue-standards.md, The labels a repository provides: the five kinds, Epic:Ops:Process, and
# every Thread:<Name> whose members reach it. A thread crosses repositories by design, so a thread
# label present in one configured repository is expected in all of them -- otherwise nothing there
# can join. Epic:<Name> labels are per-repository by definition and are not synced.

REQUIRED_EVERYWHERE = ["epic", "feature", "task", "bug", "chore", "Epic:Ops:Process"]

def label_inventory(repos):
    """Every label per repository with its colour and description -- one GraphQL request; first 100 per repository."""
    parts = []
    for i, repo in enumerate(repos):
        owner, name = repo.split("/")
        parts.append("r" + str(i) + ": repository(owner: \"" + owner + "\", name: \"" + name + "\") { nameWithOwner labels(first: 100) { totalCount nodes { name color description } } }")
    r = shell.exec(command = "gh api graphql -f query='{ " + " ".join(parts) + " }'")
    data = json.decode(data = r.stdout)["data"]
    out = {}
    for k in data:
        node = data[k]
        if int(node["labels"]["totalCount"]) > 100:
            warn(node["nameWithOwner"] + " has more than 100 labels; only the first 100 were read")
        out[node["nameWithOwner"]] = {label["name"]: {"color": label["color"], "description": label["description"] or ""} for label in node["labels"]["nodes"]}
    return out

def canonical_labels(inv, repos):
    """The expected set and its canonical colour and description: the kinds, Epic:Ops:Process, and every thread label, each as first seen in configuration order."""
    expected = {}
    names = list(REQUIRED_EVERYWHERE)
    threads = {}
    for repo in repos:
        for name in inv.get(repo, {}):
            if name.startswith("Thread:"):
                threads[name] = True
    names.extend(sorted(threads.keys()))
    for name in names:
        for repo in repos:
            if name in inv.get(repo, {}):
                expected[name] = inv[repo][name]
                break
        if name not in expected:
            expected[name] = None
    return expected

def label_faults(inv, repos):
    """Per repository: missing labels, and labels whose colour or description differs from the canonical."""
    expected = canonical_labels(inv, repos)
    rows = []
    for repo in repos:
        have = inv.get(repo, {})
        for name in sorted(expected.keys()):
            want = expected[name]
            if want == None:
                rows.append({"repo": repo, "label": name, "status": "absent everywhere", "color": "", "description": ""})
            elif name not in have:
                rows.append({"repo": repo, "label": name, "status": "missing", "color": want["color"], "description": want["description"]})
            elif have[name]["color"] != want["color"] or have[name]["description"] != want["description"]:
                rows.append({"repo": repo, "label": name, "status": "divergent", "color": want["color"], "description": want["description"], "actual_color": have[name]["color"], "actual_description": have[name]["description"]})
    return rows

def label_audit_lines(rows, repos):
    """The audit as a document: one section per repository, or one line when clean."""
    out = ["# Label audit\n", "Repositories: " + ", ".join(repos) + "."]
    if len(rows) == 0:
        out.append("\nEvery repository carries the five kinds, Epic:Ops:Process, and every thread label.")
        return out
    for repo in repos:
        mine = [r for r in rows if r["repo"] == repo]
        if len(mine) == 0:
            continue
        out.append("\n## " + repo + "\n")
        out.append("| Label | Status | Canonical |")
        out.append("|---|---|---|")
        for r in mine:
            canon = ("#" + r["color"] + " " + r["description"]) if r["color"] else ""
            out.append("| `" + r["label"] + "` | " + r["status"] + " | " + cell(canon) + " |")
    return out

def _sh_quote(s):
    """Single-quote a string for sh -c."""
    return "'" + s.replace("'", "'\\''") + "'"

def label_sync(rows, dry_run):
    """Create missing labels and align divergent ones, per the audit rows; never delete. Returns what was done, or would be."""
    done = []
    for r in rows:
        if r["status"] == "missing":
            cmd = "gh label create -R " + r["repo"] + " " + _sh_quote(r["label"]) + " --color " + r["color"] + " --description " + _sh_quote(r["description"])
            action = "create"
        elif r["status"] == "divergent":
            cmd = "gh label edit -R " + r["repo"] + " " + _sh_quote(r["label"]) + " --color " + r["color"] + " --description " + _sh_quote(r["description"])
            action = "align"
        else:
            done.append({"repo": r["repo"], "label": r["label"], "action": "skip", "reason": r["status"]})
            continue
        if dry_run:
            note("would " + action + " " + r["label"] + " in " + r["repo"])
            done.append({"repo": r["repo"], "label": r["label"], "action": action, "dry_run": True})
        else:
            shell.exec(command = cmd)
            note(action + "d " + r["label"] + " in " + r["repo"])
            done.append({"repo": r["repo"], "label": r["label"], "action": action, "dry_run": False})
    return done

# ── schedules ───────────────────────────────────────────────────────────────
#
# A schedule says what we are executing, in what order, and what is next (issue-standards.md, Three
# ways to slice the work). Lane, item, next and waits-on are declared in a Schedule: issue's table;
# progress is derived here from the feature rollup. A lane names an epic or a feature.

def schedule_issues(all):
    """Every schedule issue in the set: kind chore, title beginning 'Schedule:', by number."""
    return by_number([i for i in all if is_schedule_issue(i)])

def schedule_name(sched):
    """The name after 'Schedule:' in the title."""
    return sched["title"][len("Schedule:"):].strip()

def _short_refs(text, repos):
    """owner/repo#N rendered as repo#N for the configured repositories."""
    for repo in repos:
        text = text.replace(repo + "#", short(repo) + "#")
    return text

def lane_table(sched):
    """(lane, item spec, next, waits-on) per row whose first cell is a lane number. The item is the Epic:<Name> token if present, else the first issue reference."""
    lanes = []
    for raw in (sched.get("body", "") or "").split("\n"):
        cells = [c.strip() for c in raw.strip().strip("|").split("|")]
        if len(cells) < 2 or not regex.match(pattern = "^[0-9]+$", text = cells[0]):
            continue
        item = cells[1]
        epic = regex.find_submatch(pattern = "Epic:([A-Za-z0-9:_.-]+)", text = item)
        ref = regex.find_submatch(pattern = "(?:([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+))?#([0-9]+)", text = item)
        if epic != None and len(epic) > 1:
            spec = {"epic": epic[1]}
        elif ref != None and len(ref) > 2:
            spec = {"key": (ref[1] if ref[1] else sched["repo"]) + "#" + ref[2]}
        else:
            spec = {"unresolved": item}
        lanes.append({
            "lane": int(cells[0]),
            "spec": spec,
            "item_text": item,
            "next": cells[2] if len(cells) > 2 else "",
            "waits_on": cells[3] if len(cells) > 3 else "",
        })
    return lanes

def _aggregate(frows):
    """Sum a set of feature rows into done, open, feature count, and a state."""
    done = 0
    open_ = 0
    n = 0
    for r in frows:
        if r["feature"] == None:
            continue
        n += 1
        done += r["done"]
        open_ += r["open"]
    if n == 0:
        return {"done": 0, "open": 0, "features": 0, "state": "awaiting plan and design"}
    total = done + open_
    state = "no tasks filed" if total == 0 else (str(done) + " of " + str(total) + " done across " + str(n) + " feature(s)")
    return {"done": done, "open": open_, "features": n, "state": state}

def resolve_lane(spec, all):
    """The issue a lane names and its live progress. An epic aggregates its features; a feature rolls up its children; anything else reports its own state."""
    it = None
    if "epic" in spec:
        tag = "Epic:" + spec["epic"]
        epics = [i for i in all if tagged(i, "epic") and epic_tag(i) == tag]
        it = epics[0] if epics else None
        if it == None:
            return None, {"done": 0, "open": 0, "features": 0, "state": "no epic issue carries " + tag}
        return it, _aggregate(feature_rows([it], all))
    if "key" in spec:
        hits = [i for i in all if key(i) == spec["key"]]
        it = hits[0] if hits else None
        if it == None:
            return None, {"done": 0, "open": 0, "features": 0, "state": spec["key"] + " is not in the configured repositories"}
        if tagged(it, "epic"):
            return it, _aggregate(feature_rows([it], all))
        if tagged(it, "feature"):
            tag = epic_tag(it)
            tasks = [i for i in all if is_child(i) and epic_tag(i) == tag]
            kids = children_of(it, tasks)
            done = len([k for k in kids if k["state"] == "CLOSED"])
            return it, {"done": done, "open": len(kids) - done, "features": 1, "state": feature_state(it, kids)}
        return it, {"done": 1 if it["state"] == "CLOSED" else 0, "open": 0 if it["state"] == "CLOSED" else 1, "features": 0, "state": "closed" if it["state"] == "CLOSED" else "open"}
    return None, {"done": 0, "open": 0, "features": 0, "state": "unresolved: " + spec.get("unresolved", "")}

def schedule_rows(sched, all, repos):
    """The lanes of one schedule, in declared order, with live progress."""
    rows = []
    for lane in lane_table(sched):
        it, prog = resolve_lane(lane["spec"], all)
        rows.append({
            "schedule": schedule_name(sched),
            "lane": lane["lane"],
            "item_ref": it["ref"] if it else "",
            "item_url": it["url"] if it else "",
            "item_title": cell(it["title"]) if it else lane["item_text"],
            "item_kind": kind_of(it) if it else "",
            "done": prog["done"],
            "open": prog["open"],
            "features": prog["features"],
            "state": prog["state"],
            "next": _short_refs(lane["next"], repos),
            "waits_on": _short_refs(lane["waits_on"], repos),
        })
    return rows

def schedule_table_lines(rows):
    """Lane, item, progress, next, waits on."""
    out = ["| Lane | Item | Progress | Next | Waits on |", "|---|---|---|---|---|"]
    for r in rows:
        item = ("[" + r["item_ref"] + "](" + r["item_url"] + ") " if r["item_ref"] else "") + r["item_title"]
        out.append("| " + str(r["lane"]) + " | " + item + " | " + r["state"] + " | " + cell(r["next"]) + " | " + cell(r["waits_on"]) + " |")
    return out
