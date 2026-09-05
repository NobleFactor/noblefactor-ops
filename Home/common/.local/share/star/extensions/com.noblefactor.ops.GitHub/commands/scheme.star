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
    if len(e) == 0:
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
    for r in rows:
        r["epic"] = e["number"]
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
